[🇺🇸 English](../../adr/015-egm-nativo.md) | 🇧🇷 Português

# ADR-015: Reimplementação Nativa do Elephant-Goldfish Model (`tb egm`)

## Status

accepted (2026-11-19). O spike R0 da [spec do núcleo](../specs/egm-nativo.md) rodou ([dogfood #30–#34](../campo/dogfood.md)) e revisou o desenho de matching e convergência; a Fase A está implementada (PRs [#115](https://github.com/leonardo-moraes360/toolbelt/pull/115), [#116](https://github.com/leonardo-moraes360/toolbelt/pull/116) e [#117](https://github.com/leonardo-moraes360/toolbelt/pull/117)). O item 8 do R0 (git sobre um repositório de outro uid, que precisa de um segundo usuário) continua aberto. A [spec de confiança](../specs/egm-trust.md) segue em rascunho.

## Contexto

O [ADR-007](007-inspiracao-elephant-goldfish.md) adotou o padrão Elephant-Goldfish e deixou em aberto a pergunta "integrar o EGM ou reimplementar?". Os papéis do padrão são estes:

- **elephant**: o agente ou humano que escreveu o documento e conhece todo o contexto;
- **goldfish**: o revisor que vê só o documento e as fontes, sem histórico.

Como está hoje:

- `tb review` chama de "goldfish" três checkers **determinísticos** em Go (`scope`, `quality`, `acceptance`) que só olham um **diff** contra `rd.md`. Não revisa documento, não usa LLM e não tem loop de convergência.
- `tb drift` compara um diff com o escopo do ticket, não docs com código.
- O upstream ([google/elephant-goldfish-model](https://github.com/google/elephant-goldfish-model), Apache-2.0, v0.1.1) tem 7 skills (`brainstorm`, `draft`, `review`, `review-loop`, `resolve`, `bootstrap`, `drift-report`), 8 personas revisoras e um sintetizador, tudo em **prosa para o LLM**:
  - o contexto fresco é garantido só por instrução ao harness;
  - os findings são texto livre (Blocker/Major/Nit);
  - o estado do loop vive na memória do modelo.

Problemas da Opção A do ADR-007 (chamar as skills do EGM via MCP ou plugin):

1. **O isolamento depende do harness.** Sem subagente, a revisão para. Em Codex e Antigravity, o `AGENTS.md` do host vaza para o revisor (limitação documentada no upstream).
2. **Não há contrato verificável.** Finding em prosa não alimenta `tb stack`, `tb guard` nem o MCP.
3. **A convergência depende da memória do modelo.** As regras settled/fixed e "2 rodadas limpas" se perdem na compactação, o que contraria o [ADR-010](010-invariantes-fora-compactacao.md).
4. **As invariantes do tb não são aplicáveis lá dentro**: tb 100% local, segredo nunca em prompt, sandbox fail-closed, implementador ≠ avaliador.

## Decisão

Reimplementar o EGM **nativamente** (Opção B do ADR-007) como o grupo `tb egm`, dividindo a responsabilidade assim:

- **o tb** monta o prompt, isola e executa os goldfish, valida, casa os findings, guarda o estado e decide a convergência;
- **o LLM** só lê e julga.

A Fase A tem duas specs:
- **[Núcleo](../specs/egm-nativo.md)** (Fase A): isolamento, revisão, loop, resolve e árbitro, mais as defesas contra **conteúdo de terceiros** (repositório clonado, arquivo baixado, o próprio documento revisado). O núcleo parte de um elephant honesto, mas desleixado.
- **[Trust](../specs/egm-trust.md)** (Fase A.2, depois do spike R0): defesas contra **fraude ativa** do elephant como o mesmo usuário.

Essa divisão veio depois de 8 rodadas de revisão de uma spec única. O núcleo passou depois por mais 5 rodadas e foi congelado para o spike R0, com o desenho de matching e convergência marcado como provisório até os dados do spike; em 2026-11-19 foi revisto com esses dados (R18–R27 da spec, [dogfood #31 e #32](../campo/dogfood.md)). Os findings anti-fraude cresciam a cada rodada, inclusive sobre as próprias defesas, e impediam a convergência do restante.

### 1. Goldfish é um processo isolado e sem ferramentas

Cada goldfish é um processo **one-shot**, sem estado, iniciado a partir de `egm.agent_cmd` com `sandbox.New`. Ele **não** herda o `router.agent_cmd`, que tem outro contrato: agente escritor, com o ticket passado por env.

- **Sem nenhuma ferramenta.** As flags do perfil de vendor desligam todas as ferramentas.
  - O prompt entra por **stdin**, e os findings saem por **stdout**, separado do stderr.
  - A cada execução, o tb confere no envelope de saída que nenhuma ferramenta foi usada.
  - `bypassPermissions` e equivalentes são recusados, como no ADR-013.
- **Sandbox.**
  - **Novos modos:** workdir só-leitura, `TMPDIR` privado, HOME e `PATH` próprios, stdin e stdout separados. Com esses campos zerados, nada muda.
  - **Workdir:** é o pacote do goldfish, fora do repositório.
  - **Leituras de sistema:** as que já existiam (`/etc`, `/proc`…) continuam liberadas. Ler o environ de outro processo passa pela checagem de ptrace, que a Landlock restringe fora do domínio; o spike confirma. A ausência de ferramentas é uma segunda barreira.
- **Env mínimo.** O env é montado do zero. Variáveis que injetam código no runtime (`NODE_OPTIONS`, `PYTHONPATH`, `LD_*`…) são recusadas.
- **HOME descartável.** Diretório `0700` fora do repositório, apagado no fim. Assim o `CLAUDE.md`, as memórias e os settings globais do vendor não chegam ao goldfish. A autenticação é uma API key via `egm.env.pass` ou, com o perfil `claude`, uma cópia só da chave `claudeAiOauth` do arquivo de credencial do vendor no HOME descartável (`CLAUDE_CONFIG_DIR`). O tb se recusa a começar (`auth_expired`) quando faltam menos de 10 minutos para o access token vencer, mais que o timeout de um goldfish, então nenhum goldfish faz refresh da cópia e a credencial real nunca é rotacionada por ela.

### 2. Conteúdo de terceiros não executa nada nem vaza nada

- **Chaves de execução e segredo** (agente, `PATH`, env, HOME, vendor…) vêm **só** da config de usuário. Se aparecerem no `.toolbelt/config.yaml` de um repositório, são ignoradas com aviso.
- **Chaves de limite** (orçamento, rodadas, paralelismo, timeout, tamanhos) só podem ser **baixadas** pelo repositório, e as **de política** (rodadas limpas, atestação, matching, cobertura) só podem ser **endurecidas**. Assim, um repositório não consegue gastar créditos, travar a execução nem afrouxar a revisão.
- **A config do repositório** é lida com leitor próprio: arquivo regular, sem symlink no caminho, até 64 KiB.
- **Git endurecido e restrito.**
  - **Lista fechada:** o egm usa só subcomandos que não executam filtros, drivers nem submódulos. Não usa `git status` e nunca varre todos os arquivos versionados.
  - **Raiz:** é localizada pelo filesystem a partir do documento, sem passar do `$HOME`, com `GIT_DIR` e `GIT_WORK_TREE` explícitos. Assim, um `core.worktree` hostil não desloca a raiz. Um documento baixado sem `.git` dentro de um repositório do usuário é revisado como parte desse repositório, com as fontes limitadas pelas regras de fonte.
  - **Inspeção prévia do `.git`:** antes de chamar o git, o tb confere o `.git` direto no filesystem. Recusa symlinks, FIFOs, `commondir` desviado, `alternates` e `gitdir:` sem ponteiro de volta.
  - **Config do git com lista de fatais:** o `.git/config` é lido pelo próprio `git config --no-includes --list`, que não executa nada, para não haver diferença de interpretação entre parsers. Cada chave cai num de três grupos:
    - **fatais:** as que as opções `-c` não conseguem neutralizar (`include`, `partialclone`, `promisor`, `extensions` desconhecidas e arquivos `alternates`);
    - **neutralizadas:** as que as opções `-c` do tb sobrescrevem ou que nenhum subcomando permitido lê. São aceitas com aviso;
    - **todas as demais:** aceitas.

    A segurança vem da lista fechada de subcomandos, das opções `-c` e do env, e de testes de sentinela que provam que nada é executado, e não de enumerar chaves perigosas. Repositórios comuns funcionam (husky, git-lfs, sparse-checkout, actions/checkout). Somam-se `GIT_NO_LAZY_FETCH`, `protocol.allow=never` e um timeout por chamada.
  - **Escopo:** o router e o merge não mudam. Os loaders que o `tb stack` já roda antes de qualquer op também ficam como estão, o que é declarado fora do escopo.
- **Fontes por caminho real.** As fontes são só arquivos regulares versionados; o documento revisado pode não estar versionado.
  - Cada componente do caminho é conferido, então um diretório-pai symlink exclui a fonte.
  - Uma deny-list de segredos vale antes de tudo, e a redação ampliada preserva a contagem de linhas.
  - Outros documentos sob gate e os espelhos `docs/pt-BR/` não entram como fonte.

### 3. O documento revisado é dado não confiável

O documento é escrito pelo próprio elephant, então a injeção mais provável vem dele. As mitigações:

- **Nonce.** Delimita os blocos de dado e precisa ser ecoado na saída. Só prova que a saída veio daquela execução; **não** impede que o modelo obedeça a uma instrução injetada.
- **Atestação positiva.** Cada revisor devolve trechos literais do documento, que o tb confere contra o snapshot. O mínimo se ajusta ao tamanho do range, e ao menos um trecho precisa ser do corpo, não de heading.
- **Scan anti-injeção.** É determinístico e varre o documento, as fontes e o bloco de reavaliação, porque o modelo lê tudo isso. A lista inicial de padrões é fechada e publicada na spec (`ignore previous`, `return no findings`…).
  Os matches de cada seção viram um único finding `major` contestável.
  Na contestação, um goldfish novo decide se o trecho é discussão ou diretiva.
- **Árbitro protegido.**
  - O trecho sinalizado chega escapado e rotulado.
  - O árbitro vê o finding original e atesta o que leu.
  - Termos dirigidos ao próprio árbitro perto do match recusam a contestação.
  - Para blocker e major com revisor dono LLM, a retirada só vale se o dono concordar na rodada seguinte. Findings de checker e do scan são retirados direto.
- **Texto do elephant fora do prompt.** A reavaliação recebe só campos fixos dos findings. Notas e razões escritas pelo elephant não entram.

Isso segue o modelo do [ADR-013](013-barramento-multi-vendor.md), que já trata conteúdo de par como dado.

### 4. Dois motores, tipos de documento e um contrato

| Motor | Quando | Cobertura |
|-------|--------|-----------|
| `llm` | `egm.agent_cmd` na config de usuário | com plano detalhado: 9 revisores (8 personas; `style` roda em dois ranges). Sem plano: os 6 restantes |
| `deterministic` | sem agente ou `--engine deterministic` | `refs-exist` → `codebase-details`, `required-sections` → `structural`, `plan-files-listed` → `implementation-readiness`, todos **parciais** |

- Os checkers e o scan rodam **nos dois motores**, com severidade definida.
- Uma persona sem agente fica `unavailable`, nunca vira "zero findings".
- O tipo (`spec` ou `adr`, por glob) define o standard estrutural. A presença do plano detalhado define os revisores obrigatórios.
- Com o motor `llm`, um ADR pode chegar a `PASS`; uma spec sem plano chega no máximo a `PASS_PARTIAL`.
- **Sem agente, nada chega a `PASS`.** O motor determinístico deixa as personas LLM `unavailable`, então a cobertura é sempre parcial e o melhor veredito de qualquer documento é `PASS_PARTIAL`. Quem usa offline recebe os checkers e o scan, e o gate passa com aviso; `--strict` ou `egm.require_full_coverage` o fazem falhar.
- `review.Finding`, `tb review` e `tb drift` **não mudam**.

### 5. Estado e convergência fora do LLM, ligados ao conteúdo

O estado fica fora de qualquer worktree (`<state>/egm/<repo-id>/<slug>/`). É gravado de forma atômica, com lock liberado pelo kernel, e guarda:

- rodadas, com hashes do documento e das fontes;
- linhagens de finding com ID estável (`F-<k>`);
- **registros de convergência por conteúdo exato**.

Criar e remover worktrees não afeta o estado.

- **Reavaliação pelo revisor dono.** Cada linhagem tem um único dono: o revisor da ocorrência de maior severidade, preferindo quem vê o código.
  - O dono recebe a linhagem de volta a cada rodada e responde `still_holds`. Omitir uma linhagem invalida a saída.
  - O dono é fixo na Fase A. Se o dono não pode rodar e a linhagem bloqueia, o tb recusa a rodada em vez de rodar sem ele.
  - Um finding novo com severidade maior que uma linhagem já recusada ou encerrada vira linhagem nova, em vez de ser absorvido.
  - Linhagens de checkers fecham ou reabrem pela re-execução do checker.
- **Rodada limpa.** Nenhuma linhagem bloqueante (blocker ou major aberto, contestado, escalado ou `fixed` sem confirmação) e nenhum revisor obrigatório `not_run`.
- **Convergência.**
  - São 2 rodadas limpas seguidas **sobre o mesmo conteúdo**, com teto de `egm.max_rounds` (padrão 8) por ciclo.
  - Convergir grava um registro para aquele conteúdo e encerra o ciclo.
  - Depois do teto, o veredito é `FAIL`, e a saída é `--restart` com conteúdo novo.
- **Rebaixamentos de dois tipos.**
  - Os **de linhagem** (settle forçado, `--decline` de major) persistem enquanto a linhagem estiver naquele status. Uma edição seguida de nova convergência não os apaga. Eles só saem com `--fix` sobre uma correção real, confirmada pelo revisor dono.
  - Os **de ciclo** (`--restart`, cobertura parcial, exclusões por `max_*`…) valem só para a convergência daquele ciclo. `--restart` sempre exige conteúdo novo.
- **Vereditos**, avaliados em ordem fixa sobre o conteúdo do worktree ou, no gate, do commit:
  - `NO_STATE`;
  - `PASS`: registro para este conteúdo e esta política, sem nenhum rebaixamento e sem nenhuma linhagem bloqueante;
  - `PASS_PARTIAL`: registro com rebaixamento. O `tb stack` ganha o estado WARN para esse caso, e `--strict` ou `egm.require_full_coverage` o tornam falha;
  - `FAIL`: o ciclo bateu `max_rounds` sem convergir;
  - `STALE`: há registro só para outro conteúdo ou para outra política, ou há linhagem bloqueante sobre um conteúdo já convergido;
  - `IN_PROGRESS`.

### 6. Atalhos desleixados não chegam a `PASS`

- **`--fix` exige mudança real.** Só vale se, desde a última abertura da linhagem, um diff que não seja só de espaços tocou as linhas do finding ou as linhas citadas de uma fonte. Correções em outro trecho fecham pela reavaliação do dono.
- **Contestação vai a um árbitro.** No máximo **uma** por linhagem, decidida por um goldfish **árbitro** novo, rodado no próximo `loop next`.
  - **Evidência:** precisa estar entre as fontes citadas e verificadas da linhagem, e passa de novo pelas regras de fonte.
  - **Sem agente:** não há contestação.
  - **Árbitro falhando:** depois de duas falhas seguidas do árbitro, a linhagem é escalada.
  - **Desistir:** quem contestou pode desistir com `--fix`, que exige uma edição real, como qualquer `--fix`.
  - **Se o árbitro retira** um `nit`, um finding de checker ou um do scan anti-injeção, ele vira `settled`.
  - **Se o árbitro retira** um blocker ou major de dono LLM, o finding vira `fixed`, e o dono confirma ou reabre.
  - **Se o árbitro mantém**, o finding volta a `open`.
- **Severidade não some por casamento.** Um finding novo mais grave que uma linhagem já encerrada vira linhagem nova.
- **`--decline` não serve para fugir.** Vale para `nit`. Para `major`, rebaixa o veredito. Para `blocker`, não existe.
- **O gate avalia o commit.** O op `egm` do `tb stack` verifica os documentos sob gate tocados no range do stack, pelo conteúdo em `head` e não pelo worktree. Um documento cujas fontes mudaram sem ele ser tocado gera só aviso; a detecção de deriva fica para a Fase B. Uma seleção vazia gera aviso, e uma seleção vazia com documentos sob gate não commitados falha. Um blocker resolvido com settle forçado falha o gate mesmo sem `--strict`. A escolha do range por ref de confiança fica na spec de trust.

### 7. Gates viram transições explícitas

As perguntas interativas do upstream viram subcomandos idempotentes do `tb egm resolve`, com tabela de transições explícita. O elephant pergunta ao usuário e registra a resposta. O tb nunca edita o documento nem altera severidade.

### 8. Escopo e rede

- **Núcleo (Fase A).**
  - **Comandos:** `review`, `loop next`, `resolve` e `status`, para specs e ADRs.
  - **Preset e perfis:** o preset `consolidated`, o perfil `claude` e um perfil `custom` com declaração explícita de "sem ferramentas".
  - **Personas:** embutidas, com texto próprio, sem cópia do upstream.
  - **MCP:** na Fase A, só `tb_egm_status` e `tb_egm_resolve`. `review` e `loop next` levam minutos e ficam só na CLI.
- **Trust (Fase A.2):** ver a [spec](../specs/egm-trust.md).
- **Specs seguintes:**
  - **Fase B:** `drift-report` e `bootstrap`.
  - **Fase C:** `draft` e `brainstorm`, outros presets, personas customizadas por path, perfil `codex`, override de agente por revisor, verificação de símbolos no grafo, churn e o adaptador `review.Finding` → `egm.Finding`.
- **Rede.** O `tb` continua sem falar com a rede. Quem fala com o provedor é o processo do agente. Sem agente, tudo é 100% offline.
- **Relação com o ADR-013.** Os goldfish são workers one-shot, não agentes do barramento.
- **Localização dos docs.** As specs ficam em `docs/specs/` e os ADRs em `docs/adr/`.

## Consequências

### Positivas

- O isolamento é garantido por processo, sandbox, HOME descartável, env mínimo e ausência de ferramentas verificada, sem depender do harness.
- Rodar o `tb egm` sobre um repositório de terceiros ou um arquivo baixado não executa nada e não vaza segredo, nem por config, nem por symlink, nem por config de git.
- Atalhos desleixados não viram `PASS`:
  - `--fix` sem corrigir;
  - convergência sobre textos diferentes;
  - re-sorteio do árbitro;
  - severidade nova absorvida por linhagem encerrada;
  - edição depois do `PASS`;
  - settle forçado "lavado" por uma edição.
- O veredito vale para um conteúdo exato e sobrevive a worktrees.
- A spec do núcleo ficou implementável e revisável. A parte de fraude ativa amadurece separada, com dados do spike.

### Negativas

- Mexer no `internal/sandbox`, um componente crítico de segurança.
- O egm ganha um endurecimento de git próprio. O router e o merge ficam com os deles, e a unificação fica para a spec de trust.
- **Credenciais do vendor:**
  - a cópia da chave `claudeAiOauth` cria uma janela curta de exposição local (um arquivo `0600` no HOME descartável `0700`, apagado no fim);
  - a cópia não pode ser renovada, então com menos de 10 minutos restantes o tb se recusa a rodar e o usuário roda a CLI do vendor uma vez fora do tb;
  - no macOS, a credencial fica no Keychain, uma limitação declarada.
- O scan anti-injeção é heurístico, então uma injeção sutil obedecida por todos os revisores continua sendo risco residual.
- **Na Fase A, um agente que frauda ativamente como o mesmo usuário consegue chegar a `PASS`.** O núcleo não promete nada contra isso, e o gate é local.
- Se o spike mostrar que a Landlock não bloqueia `/proc/<pid>/environ` de outro processo, a proteção do environ do tb passa a depender só de o vendor respeitar "sem ferramentas", o que o tb verifica no envelope.
- Tocar um documento sob gate exige revisá-lo, inclusive ADRs antigos.
- A convergência com 9 revisores não determinísticos é rara em rascunhos: no R0 nenhum dos documentos reais convergiu em 4–6 rodadas, porque cada edição trazia findings novos ([dogfood #32 e #33](../campo/dogfood.md)). A Fase A mira documentos que já passaram do rascunho, mantém `max_rounds: 8` e reporta `capped` em vez de prometer convergência.
- **Custo:** cerca de 9 execuções de agente por rodada (cerca de US$ 2–2,5 com `claude`), limitadas por orçamento.
- Os perfis de vendor dependem de flags e envelopes de CLIs de terceiros, que mudam.
- Há personas próprias para manter.

## Alternativas Consideradas

### Opção A: skills do EGM via MCP ou plugin

- Aproveita o upstream pronto.
- **Não escolhida** pelos problemas 1 a 4 do Contexto.

### Só endurecer as heurísticas atuais

- Custo zero de LLM.
- **Não escolhida**: não revisa documentos, e o usuário pediu o EGM completo.

### Allowlist só-leitura do HOME real

- **Não escolhida**: o `CLAUDE.md` global e as memórias do vendor contaminam o goldfish.

### Goldfish fora do sandbox (`--allow-unsandboxed`)

- **Não escolhida**: viola o sandbox fail-closed.

### Uma spec única com núcleo e anti-fraude

- **Tentada e abandonada** depois de 8 rodadas de revisão sem nenhuma rodada limpa. Cada rodada achava ataques novos de quem tem shell como o mesmo usuário, inclusive contra as defesas recém-adicionadas, e um blocker veio de uma correção da rodada anterior.

### Unificar o endurecimento de git de router, merge e egm agora

- **Não escolhida para a Fase A**: a união das listas muda o comportamento do merge, que deixaria de ver gitlinks na checagem de caminhos sensíveis, e quebra o env do router.

### Detectar renomeação e cópia por impressão de conteúdo

- **Não escolhida**: um documento renomeado ou copiado começa sem estado e precisa convergir do zero. A detecção não protegia nenhum `PASS`.

## Referências

- [Spec: EGM nativo, Fase A, núcleo](../specs/egm-nativo.md)
- [Spec: EGM trust, Fase A.2](../specs/egm-trust.md)
- [ADR-004: Sandbox](004-sandbox-isolamento.md)
- [ADR-005: Anti-trapaça](005-anti-trapaca.md)
- [ADR-007: Inspiração Elephant-Goldfish](007-inspiracao-elephant-goldfish.md)
- [ADR-009: Router e Workers](009-router-e-workers.md)
- [ADR-010: Invariantes fora da compactação](010-invariantes-fora-compactacao.md)
- [ADR-013: Barramento multi-vendor](013-barramento-multi-vendor.md)
