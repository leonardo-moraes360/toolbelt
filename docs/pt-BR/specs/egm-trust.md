[🇺🇸 English](../../specs/egm-trust.md) | 🇧🇷 Português

# Spec: EGM trust, Fase A.2 (CI como fronteira de confiança)

Autor: toolbelt · Status: Rascunho, rodada 2 (reescrito depois da Fase A, com uma revisão em contexto novo aplicada); fatias 1 (recibo) e 2 (modo CI) de implementação feitas · Data: 2026-11-19 · Decisão: [ADR-015](../adr/015-egm-nativo.md) · Depende de: [núcleo](egm-nativo.md) (Fase A, implementada)

## Visão geral

O [núcleo](egm-nativo.md) assume um elephant honesto, mas desleixado. Um elephant que **frauda ativamente** como o mesmo usuário consegue chegar a `PASS` na Fase A: pode apontar `egm.agent_cmd` para um script, apagar o estado ou escolher o range do gate.

O primeiro rascunho desta spec respondia com nove defesas locais: pins, confirmação por TTY, uma âncora com cadeia de hash, uma ref de confiança e outras. Um processo do mesmo usuário contorna todas. Na medição da Fase A ([dogfood #32 e #33](../campo/dogfood.md)), o rascunho cresceu de 73 para 193 linhas e nunca convergiu.

Esta versão tira a fronteira de confiança **da máquina do desenvolvedor e a coloca no CI**. O CI roda com as próprias credenciais, e o workflow e a config dele não mudam sem revisão humana. O gate local fica só consultivo. A única peça local nova é um **recibo**: um arquivo commitado junto da revisão que lista as alegações do autor sobre os findings, e que o CI confere.

## Problema

A Fase A deixa três lacunas:

- **Nenhum sinal no CI.** O estado é privado (`$XDG_STATE_HOME`), então o gate num checkout de CI dá `NO_STATE` e `ok: false` (R33a). O CI, portanto, ou pula o op `egm` ou roda o loop inteiro de novo.
- **O resultado local é autodeclarado.** Tudo o que o `status` lê (estado, config, binário do agente, refs) pode ser editado pelo processo que está sendo julgado.
- **Quem chama escolhe o range.** O `tb stack --base` decide quais documentos são verificados.

## Objetivos

- **T-G1:** um documento sob gate que entra no merge passou por pelo menos uma rodada de revisão que o elephant não rodou. O elephant também não escolheu a config nem a credencial dessa rodada.
- **T-G2:** o range, o conjunto de documentos e o conteúdo verificado vêm todos do evento do CI e do commit, nunca do elephant.
- **T-G3:** toda alegação do elephant fica visível no diff (o recibo) e é reverificada pelo CI ou falha fechado. O que continua possível fica listado no Limite declarado.
- **T-G4:** o custo no CI é limitado e conhecido de antemão: uma rodada por documento selecionado, mais no máximo uma rodada do árbitro por finding que o recibo diz ter sido retirado pelo árbitro.

## Não-objetivos

- Defesas locais contra um processo do mesmo usuário (ver Mecanismos descartados).
- Rodar os revisores sob outro usuário ou numa VM.
- Unificar o endurecimento de git de router, merge e egm (`internal/gitsafe`).
- Detecção de drift em documentos que não foram tocados (Fase B).
- Provedores além do GitHub Actions. Os requisitos não dependem de provedor, mas só sai um workflow de exemplo.

## Jornadas de usuário (CUJs)

- **CUJ-T1.** *Dada* uma spec que convergiu localmente, com o recibo commitado, *quando* o PR roda o CI, *então* o CI confere o recibo contra o commit e roda uma rodada nova. Se ela não achar nada de `major` para cima, a verificação passa.
- **CUJ-T2.** *Dado* um agente que apagou o estado e escreveu o recibo à mão, *quando* o CI roda, *então* a rodada nova revisa o documento mesmo assim. Um `major` que o recibo não explica reprova a verificação.
- **CUJ-T3.** *Dado* um recibo que diz que o árbitro retirou um `major`, *quando* a rodada nova do CI acha o mesmo problema de novo, *então* o CI roda o próprio árbitro sobre ele. Se esse árbitro responder `uphold`, a verificação reprova.
- **CUJ-T4.** *Dado* um PR que edita `.github/workflows/egm.yml` para pular o passo, *quando* o PR é aberto, *então* o CODEOWNERS exige revisão humana desse arquivo. Essa proteção vem da política do repositório, não do tb.

## Plano técnico

### Requisitos

- **T-R1 (recibo).** `tb egm receipt <doc>` grava `.toolbelt/egm/receipts/<slug>.json` (o slug do R14), mais a ferramenta MCP `tb_egm_receipt`.
  - **Quando roda:** só se o `status` no worktree for `PASS` ou `PASS_PARTIAL`, e o documento e as fontes incluídas forem iguais aos blobs em `HEAD` (senão `receipt_uncommitted`, porque o CI recalcula os hashes a partir do commit).
  - **Formato:** chaves ordenadas e quebra de linha no fim. Não contém prompts nem credenciais, e as mensagens passam pela redação do R8.
  - **Campos:** `version`, `doc`, `doc_sha256`, `sources` (caminho → sha256, os mesmos da última rodada) e `claims`.
  - **O que entra em `claims`:** toda linhagem de severidade `major` para cima que se encaixe num de dois casos. No primeiro, o árbitro a retirou (o R29 leva um `major` ou `blocker` retirado a `fixed`) e ela não foi aberta nem reaberta depois. A confirmação posterior do dono mantém a alegação. No segundo, o status dela é `declined` ou `settled` com downgrade `forced` ou `forced_blocker`.
  - **Cada alegação traz:** `id`, `severity`, `kind` (`arbiter_withdrawn`, `declined` ou `forced`), `heading`, `lines` e `message`. As `lines` se referem ao documento em `doc_sha256`. Uma alegação sem `lines` é inválida.
  - **O que fica de fora:** as outras linhagens, porque o CI nunca depende delas.

  `.toolbelt/**` já fica fora das fontes (R7 do núcleo), então um recibo nunca muda o conteúdo revisado.

  Um recibo é uma alegação, não uma prova. O CI não confia em nenhum campo sem conferir (T-R2).
- **T-R2 (modo CI).** O `tb stack` ganha a flag `--egm-mode local|ci` (padrão `local`). O schema da `tb_stack` (`internal/registry/parity.go`) ganha `egm_mode`. O modo `ci` implica `--strict`, exige `-c` e roda a rodada nova com `--engine llm` (R12). Sem `-c`, falha com exit 2.
  - **Seleção:** a do R33, sobre `merge-base(--base, --head)..--head`.
  - **Ordem das verificações:** cada documento selecionado é verificado na ordem abaixo, e a primeira falha decide o resultado:
    1. **Recibo presente e válido.** O blob do recibo existe em `--head` e é lido sem erro. Toda alegação tem `lines` dentro do documento. Senão, o resultado é `no_receipt` ou `receipt_invalid`, com `ok: false`.
    2. **Recibo atual.** `doc_sha256` e `sources` batem com o que o R7a recalcula em `--head`. Senão, o resultado é `receipt_stale`, com `ok: false`.
    3. **Rodada nova.** O CI roda uma rodada com as settings do CI. O estado vai para um diretório temporário e é descartado. Se um revisor obrigatório termina `not_run`, ou o orçamento (T-R6) acaba, o resultado é `ok: false`. Caso contrário, os findings do CI de `major` para cima vão para o T-R3.
  - **Documentos apagados ou movidos:** um caminho sob gate apagado no range, ou renomeado para um caminho fora do gate (por exemplo um `README.md` de índice), gera `gated_doc_removed` e `ok: false`.
- **T-R3 (resultado da rodada nova).** Um finding do CI casa com uma alegação quando vale o `region` do R18 (mesmo heading, distância entre linhas de no máximo `match.gap`) e `sim(message) ≥ match.sim`. Dono e persona são ignorados: o CI não tem snapshots para a regra de dono do R19 nem para o mapeamento de scopes do R20, e as `lines` da alegação já se referem ao conteúdo verificado. Se a severidade do finding do CI for maior que a da alegação, ele conta como sem par (R20).

  | Finding do CI (`major` para cima) | Resultado |
  |-----------------------------------|-----------|
  | Casa com uma alegação `arbiter_withdrawn` | O CI roda o próprio árbitro sobre o finding do CI. `uphold` → `ci_upheld`, `ok: false`. `withdraw` → passa |
  | Casa com uma alegação `declined` ou `forced` | `ci_downgrade`, `ok: false`, com a alegação listada para o humano |
  | Qualquer outro caso | `ci_finding`, `ok: false` |

  O CI lista os findings abaixo de `major` em `warn`, e eles nunca reprovam a verificação. Um humano que aceita um `ci_downgrade` faz o merge pelo mecanismo de exceção do próprio repositório (T-R7), fora do tb.
- **T-R4 (CI sem credencial).** Quando o perfil do vendor não consegue autenticar, por exemplo num PR de fork sem secrets ou com `auth_expired`, o resultado é `ci_offline` e `ok: false`. A flag `--egm-allow-offline` troca isso por uma rodada só determinística, com teto `PASS_PARTIAL` e `warn`. A flag serve para workflows que aceitam esse teto.
- **T-R5 (settings do CI).** A config de usuário vem de `-c <arquivo>`. O workflow escreve esse arquivo a partir da própria definição, e o `stack.Options` o repassa ao op `egm` (hoje o op não o recebe).
  - Chaves só de usuário continuam valendo só a partir desse arquivo (R34). O `.toolbelt/config.yaml` do repositório continua podendo só apertar.
  - A credencial chega ao revisor por `egm.env.pass`, por exemplo `ANTHROPIC_API_KEY` de um secret do CI. É a primeira preferência de autenticação do spike.
- **T-R6 (custo).** `egm.ci.max_docs` (padrão 5) entra nas chaves de limite do R34, que o repositório só pode baixar.
  - **Acima do teto:** o resultado é `ci_budget` e `ok: false`, nunca um pulo silencioso.
  - **Custo por documento:** uma rodada (uma execução de agente por revisor obrigatório), mais no máximo uma rodada do árbitro por casamento com `arbiter_withdrawn`.
  - **Orçamento:** como o estado do CI é novo, o `max_agent_runs` por ciclo do núcleo (R27) limita o passo inteiro.
- **T-R7 (proteção do repositório, documentada).** `docs/examples/egm-ci.yml` traz um workflow que:
  - roda em `pull_request`;
  - instala uma release fixada do tb e confere com `sha256sum -c`;
  - escreve a config do CI em `$RUNNER_TEMP`;
  - roda `tb stack egm --egm-mode ci -c "$RUNNER_TEMP/egm.yaml" --base ${{ github.event.pull_request.base.sha }} --head ${{ github.event.pull_request.head.sha }}`.

  O `--head` é o head do PR, não o merge commit. Senão, uma mudança de fonte no lado da base deixaria todos os recibos desatualizados.

  A documentação exige estas configurações no repositório:
  - um status check obrigatório;
  - proteção da branch padrão com "include administrators";
  - CODEOWNERS em `/.github/CODEOWNERS`, `/.github/workflows/**` e `/.toolbelt/config.yaml`, com pelo menos um dono além do autor.

  Os recibos **não** ficam sob CODEOWNERS, porque o elephant os atualiza a cada mudança.
- **T-R8 (modo local).** O gate local se comporta como no R33. Ele acrescenta `"trust": "local"` ao resultado do passo, e avisos `receipt_missing` ou `receipt_stale` (nunca falhas), para que o autor veja antes do push o que o CI vai recusar.

### Mecanismos descartados

| Mecanismo do rascunho | Por que sai |
|-----------------------|-------------|
| Raiz do usuário (ignorar `XDG_*`/`HOME`) | Protege uma config local que o CI nunca lê e quebra quem usa `XDG_*` de forma legítima |
| Hashes fixados do agente e do interpretador | O CI instala o agente por conta própria, e localmente um TOCTOU (uma troca entre a verificação e o `exec`) anula o pin de qualquer jeito |
| Verificação do build do tb (`trust_dev_builds`) | O CI fixa a release do tb, e localmente a verificação só atrapalha o dogfooding |
| Cadeia de hash, âncora, trilha de auditoria, `repair` | O mesmo usuário reescreve a cadeia inteira, e o CI não consegue conferir. Forks e clones compartilham o `repo-id`, o que gera rewinds falsos |
| Confirmação por TTY | Dá para forjar um TTY, e exigir um bloqueia fluxos agênticos |
| Gate por ref de confiança | O CI tira o range do evento (T-R7), o que cobre o T-G2 sem ref local |
| Contadores de série por branch | A identidade da série com `--restart` é ambígua, e a rodada nova do CI torna os contadores locais irrelevantes |
| `internal/gitsafe` | Não tem relação com fraude. Vai para uma mudança própria |

## Modelo de ameaças (trust)

| Ação do elephant | Efeito depois da A.2 |
|------------------|----------------------|
| Aponta `agent_cmd` para um script, ou edita a config de usuário | Só o resultado local é afetado. O CI usa a própria config (T-R5) |
| Apaga ou edita o estado | O resultado local se perde ou é falsificado. A rodada nova do CI roda mesmo assim (T-R2) |
| Escreve um recibo à mão, ou inventa alegações | Os hashes são reconferidos. Uma alegação `declined` ou `forced` reprova como `ci_downgrade`, e uma `arbiter_withdrawn` passa de novo pelo árbitro (T-R3) |
| Escreve alegações sem `lines`, ou com `lines` que cobrem o documento todo | Sem `lines`, o recibo é `receipt_invalid`. Com linhas amplas, o casamento só é possível com uma alegação `declined` ou `forced` (que reprova) ou `arbiter_withdrawn` (que o CI arbitra de novo) |
| Escolhe o `--base`, ou apaga ou move o documento | A base vem do evento (T-R7). Um caminho sob gate que sai da seleção gera `gated_doc_removed` |
| Edita o workflow, o CODEOWNERS ou o `.toolbelt/config.yaml` | Exige revisão humana pelo CODEOWNERS (T-R7) |
| Usa o token `gh` do usuário para fazer merge como admin, ou para mudar a proteção da branch | O tb não impede. Com "include administrators" e um segundo dono, o merge ainda precisa desse dono. Num repositório de uma pessoa só, é possível (Limite declarado) |
| Afrouxa o documento até ele passar | Fora do escopo: o egm julga o documento como ele está, não o que ele deveria dizer |

## Limite declarado

- **A rodada nova é uma amostra só.** Um `major` que o loop local não achou e que uma rodada do CI também não acha entra no merge. A garantia é "pelo menos uma rodada independente", não convergência no CI.
- **Processo versus resultado.** Um elephant que pula o loop local, escreve um recibo sem alegações e tem uma rodada nova limpa passa. O T-G1 vale. O loop local é só um jeito de deixar essa rodada limpa.
- **A política do repositório está fora do tb.** Sem as configurações do T-R7, ou num repositório em que o único dono de código é também o admin cujo token `gh` o elephant pode usar, o elephant consegue mudar o workflow ou fazer o merge direto. O tb não detecta a falta de CODEOWNERS nem de proteção de branch.
- **Drift só nas fontes não é verificado.** Uma mudança nas fontes de um documento que não toca o próprio documento não seleciona nada. Detectar isso é da Fase B.
- **O vendor é confiável.** A credencial do CI e o modelo do vendor ficam fora do modelo de ameaças.

## Questões em aberto

- **Instabilidade.** Com que frequência uma rodada nova sobre um documento que convergiu localmente produz um `major` novo? Medir nas ADRs e specs deste repositório antes de o workflow de exemplo largar o `continue-on-error`.
- **Execuções repetidas.** O CI deve pular a rodada nova quando uma execução anterior do CI já conferiu o mesmo `doc_sha256` e as mesmas `sources`, por exemplo com um cache indexado por esses hashes? Pular troca o T-G1 por custo nas reexecuções.

## Plano de Implementação Detalhado

| Fatia | Arquivos | Requisitos |
|-------|----------|------------|
| 1 | `internal/egm/receipt.go` (criar), `cmd/tb/egm.go`, `internal/registry/egm.go` | T-R1 |
| 2 | `internal/egm/gatestack.go`, `internal/egm/ci.go` (criar), `internal/egm/config.go` (`egm.ci.max_docs`), `internal/stack/egm.go` e `stack.go` (`ConfigFile`, `EgmMode`), `cmd/tb/stack.go`, `internal/registry/parity.go` | T-R2 a T-R6, T-R8 |
| 3 | `docs/examples/egm-ci.yml`, `docs/cli.md`, a medição de instabilidade em `docs/campo/dogfood.md` (cada um com o espelho pt-BR) | T-R7, Questões em aberto |

### Plano de verificação

- **Unitário:** ida e volta do recibo com bytes determinísticos; seleção das alegações (withdraw do árbitro, decline, forced); cada verificação do T-R2 com recibos falsos (ausente, inválido, desatualizado); `gated_doc_removed`; cada linha do T-R3 com o fake agent do egm nos modos `findings`, `uphold` e `withdraw`; severidade maior contando como sem par; `ci_offline` com e sem `--egm-allow-offline`; `ci_budget`; `not_run`; a recusa com exit 2 sem `-c`.
- **e2e:** um repositório temporário com um documento convergido e o recibo, commitados, e depois `tb stack egm --egm-mode ci` com o fake agent. Um recibo escrito à mão com uma alegação `declined` que o fake agent acha de novo tem de dar `ok: false`.
- **Dogfood:** ligar o workflow de exemplo neste repositório com `continue-on-error` e registrar a taxa de instabilidade.

*Baseado no padrão EGM*
