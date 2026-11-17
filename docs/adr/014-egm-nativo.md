🇧🇷 Português | [🇺🇸 English](../en/adr/014-egm-nativo.md)

# ADR-014: Reimplementação Nativa do Elephant-Goldfish Model (`tb egm`)

## Status

proposed. Não passa a `accepted` antes do spike R0 da [spec](../specs/egm-nativo.md).

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

Reimplementar o EGM **nativamente** (Opção B do ADR-007) como o grupo `tb egm`:

- **o tb** monta o prompt, isola e executa os goldfish, valida, casa os findings, guarda o estado e decide a convergência;
- **o LLM** só lê e julga.

### 1. Goldfish é um processo isolado e sem ferramentas

Cada goldfish é um processo **one-shot**, sem estado, iniciado a partir de `egm.agent_cmd` com `sandbox.New`. Ele **não** herda o `router.agent_cmd`, que segue outro contrato: agente escritor, com o ticket passado por env. Para cada processo:

- **Sem nenhuma ferramenta.** Leitura, escrita e execução ficam desligadas pelos flags do perfil de vendor. Todo o conteúdo vai no prompt, por **stdin**, e os findings voltam por **stdout**, num canal separado do stderr. `bypassPermissions` e equivalentes são recusados, como no ADR-013.
- **Sandbox.** O sandbox ganha um modo com workdir só-leitura, `TMPDIR` privado (em vez do `/tmp` compartilhado), HOME próprio, stdin e stdout separados. Com esses campos zerados, o comportamento atual não muda.
- **HOME descartável.** É um diretório `0700` fora do repositório, apagado no fim, de modo que o `CLAUDE.md`, as memórias e os settings globais do vendor não chegam ao goldfish. A autenticação segue esta ordem de preferência, definida no spike: API key via `env.pass` do perfil, depois diretório de config do vendor apontado para o HOME descartável, e só por último a cópia de um arquivo de credencial.
- **Fontes e redação.** Entram só arquivos versionados, com uma deny-list de segredos (dotenv, chaves, certificados, arquivos ignorados pelo git) e redação ampliada que preserva a contagem de linhas.
- **Registro.** O pacote (prompt, fontes, nonce e hashes) fica persistido em `.toolbelt/egm/<slug>/round-<n>/` para auditoria.

### 2. O documento revisado é dado não confiável, e o autor é quem passa pelo gate

O documento é escrito pelo próprio elephant, que é justamente o agente sendo avaliado. Por isso a injeção mais provável vem do próprio documento. As mitigações:

- **Nonce.** Delimita os blocos de dado e precisa ser ecoado na saída. Só prova que a saída veio daquela execução; **não** impede que o modelo obedeça a uma instrução injetada.
- **Atestação positiva.** Cada revisor devolve trechos literais do documento (`checked: [{heading, quote}]`), que o tb confere contra o snapshot. Atestação abaixo do mínimo ou com trecho que não confere invalida a saída. Isso prova leitura sem penalizar rodadas limpas legítimas, ao contrário de uma heurística de "zero findings".
- **Scan anti-injeção estreito.** É determinístico e procura só frases dirigidas ao revisor ou ao modelo (`ignore previous`, `return no findings`…). Ignora código, código inline e citações, e emite um finding `major` contestável no árbitro. Documentos que apenas *discutem* revisores e findings, como esta spec, não disparam o scan; a spec é teste golden negativo. O mesmo scan roda sobre a evidência enviada ao árbitro.
- **Evidência verificada.** O tb confere cada `path:line` e cada heading citados.

Isso segue o modelo do [ADR-013](013-barramento-multi-vendor.md), que já trata conteúdo de par como dado.

### 3. Dois motores e um contrato

| Motor | Quando | Cobertura |
|-------|--------|-----------|
| `llm` | `egm.agent_cmd` executável (da fonte de confiança, §5) | 9 revisores, 8 personas (`style` roda em dois ranges) |
| `deterministic` | sem agente ou `--engine deterministic` | `refs-exist` → `codebase-details`, `required-sections` → `structural`, `plan-files-listed` → `implementation-readiness`, todos **parciais** |

- Os dois motores emitem `egm.Finding`.
- Uma persona sem agente fica `unavailable` (nunca "zero findings") e a cobertura é declarada.
- Os checkers determinísticos usam os helpers de links, paths e símbolos de `internal/graph/index_docs.go`, que passam a ser exportados.
- `review.Finding`, `tb review` e `tb drift` **não mudam**.

### 4. Estado e convergência fora do LLM

`.toolbelt/egm/<slug>/state.json` guarda os registros de rodada (hash do documento e das fontes, commit, motor, revisores, hash do argv e do binário) e as linhagens de finding com ID estável (`F-<k>`). O snapshot do documento de cada rodada fica em `round-<n>/target.md`.

- **Reavaliação pelo revisor dono.** Cada linhagem tem um único revisor dono, que a recebe de volta a cada rodada e responde `still_holds`. Omitir uma linhagem invalida a saída. Assim o fechamento não depende da sorte de o modelo "esquecer" um problema. Linhagens de checkers determinísticos e do scan fecham ou reabrem pela re-execução do checker. Findings novos são casados com as linhagens pela localização (diff entre snapshots) e pela similaridade de texto, com limiares calibrados no spike.
- **Rodada limpa.** Nenhum blocker/major em `open`, `reopened`, `contest_pending` ou `escalated`, e nenhum revisor `not_run`.
- **Convergência.** São necessárias 2 rodadas limpas seguidas, com teto de 5.
- **Vereditos.**
  - `IN_PROGRESS` enquanto o loop roda.
  - `PASS` só com cobertura completa e sem nenhuma válvula de escape.
  - `PASS_PARTIAL` quando há persona `unavailable` ou `skipped`, revisores restringidos, settle forçado, `--restart` ou rodadas além do teto. O `tb stack` ganha o estado WARN para esse caso e lista o que foi forçado; `--strict` ou `egm.require_full_coverage` o transformam em falha.
  - `FAIL` quando bate o teto sem convergir.
  - Reduzir a cobertura entre rodadas é recusado, salvo com `--narrow --reason` e confirmação humana.

### 5. Fronteira de confiança: o elephant tem shell

O elephant roda como o mesmo usuário e pode escrever no repositório. Sem proteção, bastaria apontar `egm.agent_cmd` para um script falso, baixar `clean_rounds` ou editar `state.json`. Por isso:

- **Chaves sensíveis** (`agent_cmd`, `vendor`, `tools_disabled`, `clean_rounds`, `match`, `require_full_coverage`…) só são lidas da config de usuário fora do repositório (`$XDG_CONFIG_HOME/tb/egm.yaml`) ou de `.toolbelt/config.yaml` no ref de confiança, via `git show`, como o `guard` já faz com `guard.yaml`. Um valor divergente no worktree é ignorado com aviso. `.toolbelt/config.yaml` entra nos protected paths do `guard`.
- **Registros encadeados com HMAC.** Cada rodada e cada transição levam um HMAC-SHA256 encadeado, com chave em `$XDG_STATE_HOME/tb/egm.key`, e vão também para um `audit.jsonl` append-only. O `status` e o `tb stack` recalculam o veredito a partir desses registros, não do `state.json` agregado.
- **Confirmação humana.** As válvulas de escape (`--force-settle`, `--restart`, `--narrow`, `--past-cap`) exigem que o usuário digite a confirmação num TTY interativo. São recusadas sem TTY e pelo MCP.
- **Limite declarado.** Quem tem shell como o mesmo usuário pode ler a chave HMAC, editar a config de usuário ou forjar um TTY. A Fase A torna a fraude **evidente e cara, não impossível**. Tokens de capacidade ([ADR-005](005-anti-trapaca.md) §4), que ainda não existem no código, e a execução sob outro usuário ficam para depois. Contra commit malicioso no ref de confiança, valem o CI e o review humano.

### 6. O elephant não se aprova sozinho

- **`--fix` exige mudança.** Para um blocker/major, só vale se o documento ou alguma fonte citada pela linhagem mudou desde a rodada de origem, e a reavaliação do revisor dono confirma ou reabre. Isso impede marcar como corrigido sem editar e repetir rodadas até a amostragem esquecer o finding, sem travar correções legítimas feitas no código ou em outra seção.
- **Contestação vai a um árbitro.** No máximo **uma** por linhagem, decidida por um goldfish **árbitro** novo. A evidência precisa ser uma das fontes da linhagem, estar versionada e inalterada, e é lida do `HEAD`, dentro de blocos de dado e depois do scan anti-injeção.
  - Se o árbitro retira o finding, ele vira `settled`.
  - Se o árbitro mantém, o finding volta a `open`, e a partir daí só sai por correção (com reavaliação), por `still_holds: false` do revisor dono ou por settle forçado.
- **`--dup-of` não pode baixar severidade.** A fusão preserva a maior severidade e o status mais restritivo, e não vale para linhagens já contestadas.
- **Histórico não se apaga.** `--restart` abre uma nova série de rodadas sobre o estado existente, mantendo linhagens e contestações, e rebaixa o veredito. Um documento renomeado exige `--adopt` do path antigo.
- **Settle forçado é humano.** `--force-settle --reason` exige confirmação humana, fica registrado e rebaixa o veredito para `PASS_PARTIAL`.

### 7. Gates viram transições explícitas

As perguntas interativas do upstream viram subcomandos idempotentes do `tb egm resolve`, com uma tabela de transições explícita. O elephant pergunta ao usuário e registra a resposta. O tb nunca edita o documento nem altera severidade.

### 8. Escopo e rede

- **Esta decisão e a [spec da Fase A](../specs/egm-nativo.md)** cobrem `review`, `loop next`, `resolve` e `status` com o preset `consolidated`, o perfil `claude` e um perfil `custom` com declaração explícita de "sem ferramentas", as personas embutidas (texto próprio, sem cópia do upstream) e as MCP tools `tb_egm_*`.
- **Specs seguintes:**
  - **Fase B:** `drift-report` e `bootstrap`.
  - **Fase C:** `draft` e `brainstorm`; presets `staged`, `feature` e `implementation`; personas customizadas por path; perfil `codex`; churn; adaptador `review.Finding` → `egm.Finding`.
- **Rede.** O `tb` continua sem falar com a rede. Quem fala com o provedor é o processo do agente. Sem agente, tudo funciona 100% offline.
- **Relação com o ADR-013.** Os goldfish são workers one-shot, não agentes do barramento. Um futuro papel `reviewer` no `tb bus` poderá consumir `egm.Finding`.
- **Localização dos docs.** Os documentos de design deste repositório ficam em `docs/specs/`.

## Consequências

### Positivas

- O isolamento é garantido por processo, sandbox, HOME descartável e ausência de ferramentas, sem depender do harness.
- O loop resiste às formas óbvias de autoaprovação: loteria, re-sorteio do árbitro, rebaixamento por `--dup-of`, injeção pelo próprio documento, gate sem LLM, config adulterada, estado editado à mão e `--restart`.
- Os findings estruturados alimentam `tb stack` e MCP, e a convergência é reprodutível.
- Há fallback offline com cobertura declarada.

### Negativas

- Mexer no `internal/sandbox`, um componente crítico de segurança.
- **Credenciais do vendor:**
  - a cópia de arquivo, se for a forma escolhida, cria uma janela curta de exposição local;
  - um refresh de token dentro do HOME descartável pode invalidar a credencial real, o que precisa ser testado no spike;
  - no macOS, a credencial fica no Keychain, o que é uma limitação declarada.
- O scan anti-injeção é heurístico. Uma injeção sutil que o scan não pega e que todos os revisores obedecem, ainda produzindo atestações válidas, continua sendo risco residual.
- A proteção contra o próprio elephant é evidência de adulteração, não impedimento: um processo do mesmo usuário pode ler a chave HMAC ou forjar um TTY.
- **Custo:** cerca de 9 execuções de agente por rodada, limitado por um orçamento contado em tempo real.
- Os perfis de vendor dependem de flags de CLIs de terceiros que mudam com o tempo.
- Há personas próprias para manter.

## Alternativas Consideradas

### Opção A: skills do EGM via MCP ou plugin

- Aproveita o upstream pronto.
- **Não escolhida** pelos problemas 1 a 4 do Contexto.

### Só endurecer as heurísticas atuais

- Custo zero de LLM.
- **Não escolhida**: não revisa documentos, e o usuário pediu o EGM completo.

### Allowlist só-leitura do HOME real

- É mais simples.
- **Não escolhida**: o `CLAUDE.md` global e as memórias do vendor contaminam o goldfish.

### Goldfish fora do sandbox (`--allow-unsandboxed`)

- **Não escolhida**: viola o sandbox fail-closed.

### Só o humano libera uma contestação

- **Não escolhida**: cria um gargalo humano. O árbitro fresco, com uma contestação por linhagem, preserva implementador ≠ avaliador.

### Extrair um executor comum do router

- **Não escolhida**: o executor do router é quase todo específico de ticket (workspace, escapes, commit). O `sandbox.Config` já é o caminho comum de spawn.

## Referências

- [Spec: EGM nativo, Fase A](../specs/egm-nativo.md)
- [ADR-004: Sandbox](004-sandbox-isolamento.md)
- [ADR-005: Anti-trapaça](005-anti-trapaca.md)
- [ADR-007: Inspiração Elephant-Goldfish](007-inspiracao-elephant-goldfish.md)
- [ADR-009: Router e Workers](009-router-e-workers.md)
- [ADR-010: Invariantes fora da compactação](010-invariantes-fora-compactacao.md)
- [ADR-013: Barramento multi-vendor](013-barramento-multi-vendor.md)
