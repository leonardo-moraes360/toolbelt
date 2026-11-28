[🇺🇸 English](../../specs/release-1.0.md) | 🇧🇷 Português

# Plano de prontidão da 1.0.0 — spec

Status: **rascunho** (2026-11-22; atualizado em 2026-11-27 para o `v1.0.0-rc.5`, ver G6 e O20). As decisões do dono em [§6](#6-decisões-para-o-dono) estão resolvidas.

## 1. Escopo

A fase entre v0.0.10 e v1.0.0 é **só estabilidade**: corrigir bugs, fechar lacunas de contrato, remover superfície morta, tornar a documentação verdadeira e congelar o contrato público sob semver. Nenhuma feature nova. Um item que acrescenta comportamento só entra quando corrige um problema de segurança ou substitui um contrato ambíguo que a 1.0 congelaria; cada um desses está em §6 para o dono.

Entradas: quatro auditorias somente leitura (2026-11-22), o histórico de CI da `main` (60 runs: 4 falhas, as últimas 26 verdes), as notas de campo em [dogfood](../campo/dogfood.md) (as entradas #48–#58 vêm da sessão de `tb bus` do dono em 2026-11-22) e o código em `9071d97`. As afirmações das auditorias foram conferidas por amostragem; as correções estão em [§8](#8-correções-das-auditorias).

## 2. Critérios de saída para `v1.0.0`

Tudo isto vale no commit que recebe a tag:

| # | Critério | Como se mede |
|---|----------|--------------|
| E1 | Todo item `blocker` de §4 está mergeado | as tabelas desta spec, cada linha com seu PR |
| E2 | Todo item `should` está mergeado ou adiado para a 1.1 pelo dono, por escrito, nesta spec | idem |
| E3 | 20 runs consecutivos do workflow `CI` na `main` passam, todos os jobs, na primeira tentativa (um run que precisou de re-run conta como falha) | `gh run list --branch main --workflow CI --json conclusion,attempt` |
| E4 | Zero flakes conhecidos: o job noturno de estresse (item C6) fica verde 5 noites seguidas, e nenhum teste é pulado ou apagado para esconder um flake | histórico do workflow de estresse; revisão do diff desde a v0.0.10 em busca de `t.Skip` novos |
| E5 | `golangci-lint run` limpo na `main` com o `.golangci.yml` atual; todo `//nolint` novo nomeia o linter e um motivo | o job `Lint`; `grep -rn nolint` |
| E6 | O snapshot de contrato (item R3) bate com a lista de congelamento de §3; qualquer diff posterior nele falha o CI | `TestPublicContractSnapshot` |
| E7 | A documentação afirma só o que existe: nenhum comando, flag, chave de config ou subsistema planejado descrito como presente; `TestDocs*` verdes; espelho pt-BR completo | aceite dos itens D1–D8; `go test ./cmd/tb/ -run TestDocs` |
| E8 | `v1.0.0-rc.N` passou pelo workflow de release (draft, `smoke` em 6 runners, `floors`, `publish`) e foi usada neste repositório por ≥ 7 dias sem nova entrada `high` no dogfood | run de release; [dogfood](../campo/dogfood.md) |
| E9 | `CHANGELOG.md` tem uma seção `[1.0.0]` cujo `Removed` / `Changed` lista toda mudança incompatível de §4; `SECURITY.md` aponta 1.0.x como suportada | revisão |

## 3. Lista de congelamento

Sob semver, a 1.x pode acrescentar; não pode remover nem mudar o significado de nada marcado **stable** abaixo. Mudanças incompatíveis exigem 2.0. Um item depreciado continua funcionando, escondido da ajuda, com aviso no stderr, até a 2.0.

### 3.1 Níveis de estabilidade

Todo comando Cobra e toda ferramenta MCP carrega um nível (item S7): `stable` ou `experimental`. Superfície experimental pode mudar em qualquer minor; sua linha de ajuda começa com `(experimental)` e sua descrição MCP com `[experimental]`.

| Nível | CLI | MCP |
|-------|-----|-----|
| stable | `init`, `version`, `mem *`, `verify run`, `guard check`, `review`, `drift`, `stack` (ops `verify`, `review`, `drift`, `guard`), `sandbox check\|run`, `graph *` (lexical e `--vectors`/`--mode`), `ticket *`, `workspace *`, `merge` (estratégias `fail`, `last_wins`, `semantic`), `mcp` | `tb_version`, `tb_run`, `tb_mem_*`, `tb_verify_run`, `tb_review`, `tb_drift`, `tb_guard_check`, `tb_stack`, `tb_graph_*`, `tb_ticket_*`, `tb_merge` |
| experimental | `bus *`, `router *`, `board *`, `watchdog *`, `egm *`, `vault *`, `red *`, `blue *`, `lab *`, `bench *`, `merge --strategy prompt`, ops `egm`, `vault`, `seccheck` do `stack` | `tb_bus_*`, `tb_router_status`, `tb_board`, `tb_watchdog_alerts`, `tb_egm_*`, `tb_vault_*`, `tb_red_*`, `tb_blue_*`, `tb_sec_pair` |
| removido antes da 1.0 | `guard check --strict` (decisão O1) | `tb_echo`, `tb_env` (O2); `strict` do `tb_guard_check` |
| interno (nunca público) | `__graph-embed`, `fake-agent`, comandos ocultos | — |

Motivo do conjunto experimental: seus ADRs ainda estão `proposed` (009, 013, 016, 017) ou os próprios dados de campo dizem que o desenho não assentou (convergência do egm, [dogfood](../campo/dogfood.md) #32–#34; KPIs do bench não implementados). O nivelamento é a decisão O3.

### 3.2 CLI

Congelado para comandos stable: caminhos de comando, nomes e tipos de flags, argumentos posicionais, defaults, as flags globais `--json`, `-c/--config`, `-v/--verbose`, `--version`. O inventário congelado é o snapshot da árvore Cobra (item R3), não uma lista feita à mão.

Renomeações que entram antes da 1.0 (S1–S3): ver a trilha S em §4.

### 3.3 MCP

Congelado para ferramentas stable: nome da ferramenta, nomes e tipos de parâmetros, `required`, defaults, anotações e o formato do resultado (§3.4). Servidor: JSON-RPC por stdio, versões de protocolo `2025-11-25`, `2025-06-18`, `2024-11-05` (`internal/mcp/server.go`). Total de ferramentas depois das remoções: 52 (54 hoje).

Regra de nomes congelada com a 1.0: um parâmetro MCP é a flag da CLI com `-` → `_`, no plural quando a flag é repetível (`--egm-doc` → `egm_docs`, `--check` → `checks`). Um argumento posicional da CLI vira parâmetro nomeado (`tb blue apply <class>` → `class`). Uma exceção documentada: `tb_merge` recebe `apply` (default `false`, dry-run) onde a CLI recebe `--dry-run`, porque um agente precisa optar por mover refs (decisão O5). `cwd` é opcional em toda ferramenta e por padrão é o diretório de trabalho do servidor (item S4).

### 3.4 Envelopes JSON

**Convenção (decisão O4):** todo documento JSON que o `tb` imprime no stdout com `--json`, e todo corpo de resultado de ferramenta MCP, tem um campo string de topo `v` = `<kind>/<major>.<minor>`. Bump de minor: campos acrescentados. Bump de major: qualquer outra coisa. Consumidores devem ignorar campos desconhecidos. É a forma que sete envelopes já usam; as formas inteiras `version` / `v` ficam só em arquivos em disco (§3.6).

Já conformes (mantidos na versão atual e congelados nela): `bus/0.1` (mensagem), `bus-wake/0.1`, `bus-discover/0.1`, `router-task/0.1`, `egm-source-drift/0.2`, `egm-draft/0.1`, `egm-brainstorm/0.1`.

Precisam adotá-la (item J1), começando em `<kind>/1.0`:

| Kind | Produtor |
|------|----------|
| `error/1.0` | o corpo de erro da CLI `{"error","reason","exit_code"}` (hoje só egm e vault; o item J3 faz todo comando usá-lo sob `--json`) e erros de ferramenta MCP (item J4) |
| `version/1.0` | `tb version --json`, `tb_version` |
| `init/1.0` | `tb init --json` |
| `mem/1.0` | `tb mem *`, `tb_mem_*` |
| `verify/1.0` | `tb verify run`, `tb_verify_run` (`verify.Result`) |
| `guard/1.0` | `tb guard check`, `tb_guard_check` |
| `review/1.0` | `tb review`, `tb_review` |
| `drift/1.0` | `tb drift`, `tb_drift` |
| `stack/1.0` | `tb stack`, `tb_stack` |
| `sandbox-run/1.0`, `sandbox-check/1.0` | `tb sandbox run\|check`, `tb_run` |
| `graph-<op>/1.0` | `tb graph search\|callers\|deps\|impact\|explain\|path\|stats\|index\|rebuild`, `tb_graph_*` |
| `ticket/1.0`, `ticket-list/1.0` | `tb ticket *`, `tb_ticket_*` |
| `workspace/1.0` | `tb workspace *` |
| `merge/1.0` | `tb merge`, `tb_merge` |
| kinds experimentais (mesma regra, não congelados) | `board`, `router-status`, `watchdog`, saídas `bus-*` que não são envelope de mensagem, `egm` (`egm.Outcome`), `vault-*`, `red-*`, `blue-*`, `lab-*`, `bench-*` |

### 3.5 Códigos de saída

Tabela estável proposta (item J2). Hoje o `main.go` só mapeia `verify.UsageError` para 2; egm e vault têm seus próprios 0–3; `tb sandbox run` sai com 0 em timeout e sob `--json` (bugs J5, J6).

| Código | Significado | Mudança em relação a hoje |
|--------|-------------|---------------------------|
| 0 | sucesso: `PASS`/`SKIPPED`, sem `SeverityError` (ou `--warn-only`), checagem passou | — |
| 1 | a checagem rodou e falhou (`FAIL`, `ESCALATE`, `SeverityError` do guard, check do vault, `FAIL` do egm), ou erro de execução depois de o trabalho começar | — |
| 2 | erro de uso ou recusa antes de qualquer trabalho: flag ou argumento inválido, subcomando desconhecido, `--ruler`/`--check` inválido, motivos de recusa do egm/vault | erros de parse de flag/argumento e subcomandos desconhecidos passam de 1 para 2 |
| 3 | egm: o veredito não permite a operação (`receipt` recusado salvo `PASS`/`PASS_PARTIAL`) | documentado; só egm |
| 124 | `tb sandbox run`: o filho estourou `--timeout` | novo; hoje 0 |
| 127 | um comando configurado não foi achado ou não é executável (comandos de gate do `verify`, o exec do filho em sandbox) | — |
| 130 | interrompido por SIGINT/SIGTERM (`tb mcp` hoje) | documentado |
| filho | `tb sandbox run` repassa o código não zero do filho, com ou sem `--json` | com `--json` hoje sai 0 |
| máximo | `tb stack` sai com o maior código de saída entre seus passos | documentado |

### 3.6 Chaves de config e formatos em disco

**Config** (`.toolbelt/config.yaml`, `~/.config/toolbelt/config.yaml`): toda chave documentada em [cli.md](../cli.md#configuração) na 1.0 fica congelada. Chaves que são lidas do YAML mas que nada consome (conferido: nenhum leitor fora de `internal/config`) saem antes da 1.0 (item K1, decisão O6): `defaults.sandbox`, `defaults.timeout`, `defaults.json_output`, `sandbox.allow_net`, `sandbox.cpu_limit`, `sandbox.mem_limit`, `sandbox.time_limit`, `graph.auto_index`, `graph.infer_relations`, `vault.sync_on_change`, `logging.level`, `logging.file`, `logging.format`. A decodificação continua tolerante: uma chave removida num arquivo antigo é ignorada, com um aviso de uma linha no stderr (item K2). `router.bus.claim_timeout` é usada (`internal/router/bus.go:111`) e fica. `sandbox.allow_unsandboxed` fica: é um opt-in explícito, não um fallback silencioso.

**Arquivos que o usuário escreve ou commita — congelados; a 1.x lê todo arquivo da 1.0 e nunca reescreve uma versão mais nova que não conhece:**

| Arquivo | Marcador de versão |
|---------|--------------------|
| `.toolbelt/config.yaml` | nenhum (chaves acima) |
| `.toolbelt/invariants.yaml` | `version: 1` |
| `.toolbelt/tickets/TKT-*.yaml` | nenhum hoje; o item J7 acrescenta `version: 1` opcional (ausente = 1) |
| `.toolbelt/guard.yaml`, `.toolbelt/thresholds.yaml`, `.toolbelt/screens.yaml`, `agent_docs.yaml` | nenhum (esquema no cli.md) |
| `.toolbelt/egm/receipts/<slug>.json` (feature experimental, mas commitada por usuários) | `version` 1–3 |
| `.toolbelt/secchecks.yaml` (experimental) | `version` |

**Estado local — não congelado; o tb o reconstrói, migra para frente ou recusa com um motivo nomeado, nunca o lê errado:**

| Arquivo | Regra |
|---------|-------|
| `.toolbelt/graph.json` | cache derivado; o item J7 acrescenta `version`; divergência significa reindexar (necessário para G1, que muda IDs de nós) |
| `.toolbelt/graph.vec` | magic `TBVEC1` + cabeçalho JSON `v`; divergência → `vectors_stale`, reconstrução |
| `.toolbelt/bus/bus.db` | `PRAGMA user_version` (4 hoje); a 1.x migra para frente a partir de qualquer esquema da 1.0, nunca para trás |
| router, `state.json` do egm, recibos do merge `prompt`, `$XDG_STATE_HOME/toolbelt/bus/*` | internos; recusa em versão desconhecida |

## 4. Itens de trabalho

Níveis: **blocker** (E1), **should** (E2), **nice** (pode escorregar para 1.0.x). Cada trilha tem um escopo de arquivos disjunto; um PR que precise tocar um arquivo de outra trilha diz isso na descrição e entra depois do PR aberto daquela trilha. A documentação de uma mudança de comportamento entra no mesmo PR da mudança (inglês + pt-BR); a trilha D cuida das reescritas transversais.

### Trilha C — CI e flakes

Escopo: `*_test.go` nos pacotes citados, `.github/workflows/ci.yml`, um novo `.github/workflows/stress.yml`. Nenhum código de produção.

| Id | Nível | Área | Aceite |
|----|-------|------|--------|
| C1 | blocker | `internal/mdparse/mdparse_test.go:186-197` `TestWikilinksManyOnOneLine` | a checagem de linearidade compara contagem de operações ou uma razão sobre execuções repetidas, não tempo de relógio; 50 execuções com `-race -count=50` passam num runner carregado |
| C2 | blocker | `internal/router/router_escape_test.go:237` `TestDaemonFromWorktreeCreationDoesNotFailTicket` e as outras esperas "ainda vivo" | as esperas fazem polling com prazo escalado para `-race`; mensagens de falha em inglês; 50× verde |
| C3 | blocker | `internal/router` `TestPauseThenResume` ("no router state found", `TempDir` RemoveAll "directory not empty") | o teste espera a goroutine do router sair antes da limpeza; 50× verde |
| C4 | blocker | `internal/embed` `TestClientStartFailures` (`embed_crashed: exit status 2` com `GOCOVERDIR` definido) | a falha esperada é verificada pelo motivo, independente do ambiente de cobertura; verde sob `-coverprofile` |
| C5 | should | run 44842322646: 15 testes do router com `signal: killed` no timeout de 60 s | testes do router que disparam git ganham prazo por teste e rodam com limites de `t.Parallel` que cabem num runner; o job `Test` tem `timeout-minutes` com folga |
| C6 | blocker | novo `stress.yml` (noturno + `workflow_dispatch`) | `go test -race -count=10` sobre `internal/router`, `internal/mdparse`, `internal/embed`, `internal/bus`, `internal/busrun`; é o que E4 mede |
| C7 | should | cobertura do fail-closed: `TestNewRequiresAllowUnsandboxedWithoutLandlock`, `TestRunSandboxNonLinuxFailsClosed`, `TestLabUpPodmanFailsClosedCLI` sempre pulam no runner (ele tem Landlock) | um ponto de injeção só de teste (variável não exportada definida em `_test.go`, nunca variável de ambiente ou flag) faz o caminho "sem Landlock" rodar no CI Linux; a recusa é verificada. Respeita a invariante fail-closed: nenhum bypass em runtime |
| C8 | nice | `internal/verify/verify_test.go:62`, `internal/stack/stack_test.go:379`: o fixture `TestMarkerOnlyOnHead` falha de propósito e seu `--- FAIL` aparece nos logs do CI | a mensagem do fixture diz `expected fixture failure`, então varreduras de log não o reportam |

C1–C4 e C7 tocam pacotes diferentes e rodam em paralelo; C5 e C6 tocam workflows: um PR só, ou C6 depois de C5.

### Trilha J — códigos de saída e envelopes JSON

Escopo: `cmd/tb/main.go`, `cmd/tb/sandbox.go`, um novo `cmd/tb/output.go` (impressora `--json` comum e corpo de erro), os pontos de impressão JSON em `cmd/tb/*.go` (não as declarações de flags), as structs de resultado em `internal/{verify,guard,review,stack,graph,ticket,workspace,merge,mem,version}`, `internal/registry/registry.go` (formatação do resultado). Docs: "Exit Codes" e "Errors and Usage" do [cli.md](../cli.md).

| Id | Nível | Área | Aceite |
|----|-------|------|--------|
| J1 | blocker | todo envelope de §3.4 | cada um carrega `v`; um teste de tabela lista todo comando `--json` e confere que `v` casa com `^[a-z-]+/\d+\.\d+$`; resultados MCP batem com os da CLI |
| J2 | blocker | `cmd/tb/main.go` `exitCodeFor` | a tabela de §3.5: erros de flag/argumento do Cobra e subcomandos desconhecidos saem com 2; um teste por linha; tabela do cli.md substituída (frase "não estável" removida) |
| J3 | should | `cmd/tb/output.go` | sob `--json`, o erro de todo comando é o corpo `error/1.0` no stdout com `reason` e `exit_code`, como egm e vault fazem hoje |
| J4 | should | `internal/registry/registry.go` `Call` | um erro de ferramenta é `isError: true` com o corpo `error/1.0` como texto (hoje texto puro) |
| J5 | blocker | `cmd/tb/sandbox.go:45-51` | `tb sandbox run --json` sai com o código do filho (hoje 0) |
| J6 | blocker | `cmd/tb/sandbox.go:49` | timeout sai com 124 (hoje 0); o JSON tem `timed_out: true` |
| J7 | should | `internal/graph/graph.go` (`version` no graph.json; reindexa na divergência), `internal/ticket` (`version` opcional) | arquivos antigos carregam; graph.json sem `version` dispara reconstrução, não erro |
| J8 | nice | `v` inteiro em `internal/merge/receipt.go:91` | renomeado para `version`, lendo `v` por compatibilidade (estado local, pouco valor) |

J1 e J2 conflitam nos pontos de impressão de `cmd/tb/*.go`: faça J2 + J5 + J6 primeiro (pequenos), depois J1 + J3. J4 e J7 são independentes.

### Trilha S — superfície e nomes da CLI/MCP

Escopo: declarações de flags em `cmd/tb/{guard,verify,stack}.go`, `internal/registry/{builtin,parity,egm,vault,annotations}.go`, a infraestrutura de nível de estabilidade (`Annotations` do Cobra, campo no `Tool` do registry), `cmd/tb/docs_cli_test.go` (a asserção do `--strict`), as seções correspondentes do cli.md.

| Id | Nível | Área | Aceite | Alias |
|----|-------|------|--------|-------|
| S1 | blocker | `tb guard check --strict`, o caminho do guard no `tb stack`, `strict` do MCP `tb_guard_check` (`parity.go:290`) | removidos (O1); `assertDocsGuardStrictDeprecated` passa a ser "o cli.md diz removido na 1.0"; `Removed` no CHANGELOG | nenhum: é no-op desde que foi depreciado; o `--strict` do `egm status` e do `stack` (sentido do egm) não muda |
| S2 | blocker | `internal/registry/builtin.go:29-44` `tb_echo`, `tb_env` | removidos (O2); `tb_version` ganha `cwd` só se a O2 decidir assim; tabela de anotações e testes atualizados | nenhum (ferramentas de debug) |
| S3 | blocker | `tb verify run --no-check`, `tb stack --no-check` (nomes repetíveis) ao lado de `--no-checks` (bool); MCP `skip_checks` | flag renomeada para `--skip-check`, para que a regra de §3.3 a mapeie em `skip_checks` (O7) | `--no-check` fica oculta + depreciada até a 2.0 |
| S4 | should | `internal/registry/egm.go:25,38,57`, `vault.go:23-86` | `cwd` deixa de ser `required`; default como em `resolveToolCWD` | n/a (relaxamento) |
| S5 | should | português em descrições de ferramentas (`parity.go:320` "testes ocultos") | descrições em inglês; um teste rejeita letras não ASCII em descrições de ferramentas | — |
| S6 | should | `apply` do `tb_merge` vs `--dry-run` | mantido (O5); a descrição e o cli.md dizem por quê | — |
| S7 | blocker | marcador de nível em todo comando e ferramenta (§3.1) | um teste falha se um comando ou ferramenta não tiver nível; ajuda e descrições mostram `(experimental)` / `[experimental]` | — |

S1–S3 tocam arquivos diferentes e rodam em paralelo; S7 toca a declaração de todo arquivo de comando, então entra por último na trilha, depois do rebase de S1–S3.

### Trilha G — bugs de correção achados em campo

Escopo: `internal/graph/{index_go,index_common}.go` e os helpers de ID dos outros indexadores, `internal/verify/failure_output.go`, strings de mensagem em `internal/egm/{roundctx,review}.go`, `cmd/tb/init*` e `internal/config/init_detect.go` (só reconferência dos dogfood #1/#7).

| Id | Nível | Área | Aceite |
|----|-------|------|--------|
| G1 | blocker | IDs do indexador Go (nota de [dogfood](../campo/dogfood.md) de 2026-11-22; `index_go.go:34,52,54`): `pkg:<name>` funde todo `main`; métodos perdem o receptor | o ID de pacote é o import path (ou o diretório relativo ao módulo); o ID de método inclui o receptor (`func:<pkg>.<Recv>.<Name>`); `funcIDs` indexado por receptor + nome; `version` do graph.json incrementada (J7). Incompatível para IDs guardados, então precisa entrar antes da 1.0 |
| G2 | blocker | `internal/graph/index_common.go:50` `readSource` segue symlinks e trava em FIFOs ([spec do vault](vault.md) Q5) | a varredura do grafo lê pelo mesmo leitor seguro de `vector.go:399-410` (nenhum componente symlink, arquivo regular, limite de tamanho); um FIFO em `docs/` não trava `tb graph index` (teste) |
| G3 | should | dogfood #10: `reason` do `verify` truncado pelo começo | reconferir na main; se `failure_output.go` já mantém as linhas `--- FAIL`, fechar o #10 no dogfood com o commit; senão corrigir e testar |
| G4 | should | mensagens ao usuário bilíngues ou em português (`internal/egm/roundctx.go:89`, `review.go:222`) | só inglês; testes atualizados |
| G5 | should | reconferir os dogfood #1, #7, #17 (marcados abertos; a auditoria diz corrigidos) e N1, N3, N4 da rodada 2 de campo (notas playwright-mcp, pydantic-ai) | cada linha diz `fixed in <sha>` ou fica aberta com um item G; docs atualizados nas duas línguas |

| G6 | blocker | base padrão do `tb stack` / MCP `tb_stack` ([dogfood](../campo/dogfood.md) #62, paprika 2026-11-25) | sem `--base`: `base_commit` do ticket, senão merge-base com a branch padrão, senão `HEAD~1`; JSON `base` e `base_source`; `--base HEAD~1` explícito sem mudança. Muda um padrão congelado (O20), então precisa entrar antes da 1.0: corrigido em `fix/rc5-stack-base`; a tag `v1.0.0-rc.5` parou no gate de advisories (dogfood #70), então sai como `v1.0.0-rc.6` com o llama.cpp b12658 (o soak recomeça) |

G1 e G2 tocam o mesmo pacote, mas arquivos diferentes; rode G2 primeiro (pequeno). G3–G5 são independentes.

### Trilha B — bus (sessão de campo do dono, 2026-11-22)

Escopo: `internal/bus`, `internal/busrun`, `cmd/tb/bus*.go`, `cmd/tb/mcp.go` (`bindBusAs`), `internal/registry/bus.go`, `internal/version` (só B5), [bus-protocolo.md](../bus-protocolo.md), [bus-harness.md](../bus-harness.md). O bus é experimental (§3.1), mas B1 é correção de segurança e bloqueia a 1.0 de qualquer forma. Entradas de dogfood #48–#58.

Conferido no código:

- `Store.Join` (`internal/bus/store.go:467`) exige convite; um convite vinculado faz re-join e troca o token. Mas `tb mcp --bus-as <channel>/<name>` (`cmd/tb/mcp.go:58`, `registry.BindBusSession`) carrega a sessão salva sem lock, e só o `tb bus run` pega um lock (`internal/busrun/harness.go:126`). Dois clientes iniciados com o mesmo `--bus-as` dividem uma identidade e um worktree.
- Não existe campo de dependência em lugar nenhum do bus: payloads de `task.assign` são opacos (`docs/bus-protocolo.md`), e `ClaimThread` (`internal/bus/lease.go:70`) só confere o lease. `dependencies` de ticket são respeitadas pelo `tb router`, não pelo bus.
- `tb bus invite` e a criação de canal imprimem a expiração como `15:04:05Z07:00`, sem data (`cmd/tb/bus.go:119,240`).
- `wake_mismatch` (`internal/busrun/notify.go:245`) informa a divergência, não a correção.
- O hook SessionStart que acusou a versão divergente é do dono, fora deste repositório. Do lado do tb, `tb version` imprime `dev`/`unknown` num build sem ldflags (`internal/version/version.go`), então um hook não tem nada estável para comparar além de um hash de binário ou de módulo.
- Os estados de claim (`held`/`expired`/`released`) estão no `tb_bus_status`; como um claim liberado terminou (completado, expirado, revogado, tomado) não está.

| Id | Nível | Tipo | Área | Aceite |
|----|-------|------|------|--------|
| B1 | blocker | bug (segurança) | `cmd/tb/mcp.go` `bindBusAs`, `internal/busrun/harness.go`, arquivos de sessão de `internal/bus` | um lock exclusivo por `<channel>/<name>` no diretório de estado privado, mantido por `tb mcp --bus-as` e `tb bus run` durante toda a vida; um segundo detentor sai com 2 e `session_in_use`, nomeando o pid do detentor; um re-join com convite vinculado enquanto o lock está preso é recusado do mesmo jeito; lock liberado na saída e em crash (flock) |
| B2 | should | lacuna de contrato | [bus-protocolo.md](../bus-protocolo.md), ajuda do `tb bus send` | a documentação diz que `payload` é opaco e que ordenar por dependência é tarefa do `tb router` (`dependencies` do ticket); `tb bus send` com payload de `task.assign` contendo `deps`/`dependencies` imprime um aviso no stderr de que o bus não as aplica. Dependências no bus: O8 |
| B3 | should | bug | `cmd/tb/bus.go:119,240` | expiração impressa em RFC 3339 com data, fuso local, como o `channel show` faz |
| B4 | should | bug | `internal/busrun/notify.go:245`, `check.go` | a linha de `wake_mismatch` diz qual lado mudar (o `wake` do Agent Card ou o arquivo de wake) e o comando que faz isso |
| B5 | should | lacuna de contrato | `internal/version/version.go` | sem ldflags, `Version`/`Commit` vêm de `debug.ReadBuildInfo` (`Main.Version`, `vcs.revision`, `vcs.modified`); `tb version --json` ganha `modified`; a documentação diz a regra de comparação para hooks: versão + commit, nunca um hash de módulo ou de binário |
| B6 | should | lacuna de contrato | `tb_bus_status`, `tb bus channel show` | cada claim traz `outcome` (`completed`, `expired`, `revoked`, `taken_over`, ou null enquanto preso). Aditivo, mas corrige um campo ambíguo antes de o envelope ganhar versão (O9) |
| B7 | nice | testes de regressão | `cmd/tb/bus_test.go`, `cmd/tb/mem_test.go` | testes no nível da CLI para o que funcionou: 17 `tb bus send` com chaves de idempotência enviados duas vezes guardam 17 mensagens; enviar para um agente fora do canal o nomeia (`store_test.go:206` cobre só o store); `tb mem list` num repositório novo lista as invariantes semeadas |
| — | adiado (O10–O12) | feature | — | lista de tarefas do canal com estado; reservar paths sem tarefa / liberar um claim à mão; checagem de sobreposição de paths entre tickets |

B1 primeiro (toca `mcp.go` e o harness); B3, B4, B5, B6 tocam arquivos diferentes e rodam em paralelo com ele. B2 é documentação mais um aviso em `cmd/tb/bus.go`: depois de B3.

### Trilha K — limpeza de config

Escopo: `internal/config/config.go`, `internal/config/init_detect.go`, `cmd/tb/init*.go`, seção "Configuration" do cli.md.

| Id | Nível | Área | Aceite |
|----|-------|------|--------|
| K1 | blocker | as 13 chaves mortas de §3.6 | removidas das structs e de `DefaultConfig`; `tb init` deixa de escrevê-las; o cli.md lista só chaves vivas |
| K2 | should | `config.Load` | uma chave que a struct não conhece imprime um aviso no stderr por arquivo por execução (`unknown config key graph.auto_index (ignored)`); nunca erro, para que arquivos antigos continuem funcionando |
| K3 | should | dogfood #1 (`tb init` escreve `golangci-lint run` sem checar o PATH) | reconferir; corrigir ou fechar (divide `init_detect.go`, por isso fica aqui e não em G5) |

K1 depois K2 (mesmo arquivo). K3 em paralelo.

### Trilha D — documentação que promete demais

Escopo: `README.md`, [arquitetura.md](../arquitetura.md), [benchmark.md](../benchmark.md), [identidade-visual.md](../identidade-visual.md), `docs/adr/*`, `site/index.html`, a árvore de visão geral do cli.md e as seções planejadas `tb run`/`tb proc`, o cabeçalho de [appsec-playbooks.md](appsec-playbooks.md), as notas locais do `CLAUDE.md`; cada arquivo com seu par pt-BR. As outras trilhas editam só a seção do próprio comando no cli.md.

| Id | Nível | Área | Aceite |
|----|-------|------|--------|
| D1 | blocker | arquitetura.md §3.2–3.3, §4.2–4.3, §5.1–5.2, §6, §8.2–8.3, §9.3, §13.3–13.4 | descreve o que existe: graph.json + BM25 (sem índice SQLite/FTS5, sem tabelas nodes/edges), vault lido sob demanda (sem watcher), SQLite só no `bus.db`, sem bubblewrap no `tb sandbox`, a árvore real de `.toolbelt/`; roles/CapabilityToken/`tb agent` removidos conforme o ADR-020; questões em aberto já respondidas fechadas |
| D2 | blocker | README.md L28-29, L59-70, badge | sem afirmações de bubblewrap, cgroups, "capability-based permissions" ou FTS5; badge do Go a partir do `go.mod`; total de MCP a partir do snapshot (R3) |
| D3 | blocker | seções planejadas do cli.md (`tb run`, `tb proc`, `run / proc / md / agent`, `serve`) e árvore de visão geral | comandos planejados viram uma linha do roadmap do README; a árvore lista todo comando de topo com seu nível (acrescenta `red`, `blue`, `lab`, egm `drift-report\|bootstrap\|brainstorm\|draft`) |
| D4 | should | benchmark.md | sem afirmação de p < 0,05; KPIs não implementados (`file_reads`, `rework_rate`, `guard_violations_*`, `defects_found`, `false_positive_rate`, `tokens_per_review`, `reviewer_agreement`, `wall_time_total`, `critical_path`) movidos para "Não implementado" (O13) |
| D5 | should | status dos ADRs | 003 registra que o graph.json substituiu o índice SQLite; 004 registra que o seccomp foi entregue; 010 e 018 `accepted` (entregues, estáveis); 009, 013, 016, 017 ficam `proposed` e dizem "experimental na 1.0" (O14) |
| D6 | should | `site/index.html` | sem "v0.0.5" / "36 tools" / Go 1.22; texto sem versão ou com valores da release |
| D7 | nice | identidade-visual.md:272 `.tb/vault/` → `.toolbelt/vault/`; status no cabeçalho de appsec-playbooks.md | corrigidos |
| D8 | should | "Local notes" do `CLAUDE.md` | `golangci-lint` está no PATH agora; total de ferramentas MCP a partir do snapshot; notas velhas removidas |

D1–D8 tocam arquivos disjuntos e rodam em paralelo. D3 toca o cli.md: entra antes de as trilhas J, S e K editarem suas seções do cli.md, ou elas fazem rebase.

### Trilha R — infraestrutura de release e o congelamento

Escopo: `.goreleaser.yaml`, `.github/workflows/release.yml`, [release.md](../release.md), `SECURITY.md`, `CHANGELOG.md`, `.toolbelt/invariants.yaml` (via `tb mem`), novos `cmd/tb/contract_test.go` + `internal/registry/contract_test.go` + `testdata/contract/`.

| Id | Nível | Área | Aceite |
|----|-------|------|--------|
| R1 | blocker | `.goreleaser.yaml`, passo de publish do `release.yml` | tags `-rc.N` publicam como prerelease do GitHub e não como `latest` (`release.prerelease: auto`; o publish preserva); um dry run com `goreleaser release --snapshot` mostra isso |
| R2 | blocker | a invariante `inv-1795019844137842036` ("No-CGO binary … never add a dependency that needs CGO") contradiz o ADR-018 e o `.goreleaser.yaml` (`CGO_ENABLED=1`) | editada com `tb mem edit` para a regra entregue (cgo só para o código nativo vendorizado e em allowlist do ADR-018; SQLite em Go puro continua) (O15) |
| R3 | blocker | snapshot de contrato | um teste grava a árvore Cobra (caminhos, flags, tipos, defaults, nível) e o `tools/list` do MCP (nomes, esquemas, anotações, nível) em `testdata/contract/*.json` com `-update`; o CI falha em qualquer diff; o snapshot da 1.0 é a lista de congelamento de §3.2–3.3 |
| R4 | blocker | `SECURITY.md` | versões suportadas: 1.0.x (depois, a minor mais recente da 1.x); seccomp listado como implementado; bubblewrap não afirmado |
| R5 | blocker | `CHANGELOG.md` | `[Unreleased]` → `[1.0.0-rc.1]`, depois `[1.0.0]`; `Removed` e `Changed` listam todo item incompatível (S1, S2, S3, J2, J5, J6, K1, G1) |
| R6 | should | [release.md](../release.md) | fluxo de rc (§7) documentado; exemplos usam `v1.0.0-rc.1`; política de depreciação (§3) linkada |

R3 entra depois de S1–S3, S7 e J1 (fotografa o resultado deles). R1, R2, R4, R6 a qualquer momento. R5 é o último PR antes da tag.

### Trilhas num relance

| Trilha | Escopo | Blocker | Should | Nice | Adiado |
|--------|--------|---------|--------|------|--------|
| C — CI e flakes | testes + workflows | 5 | 2 | 1 | — |
| J — códigos de saída e envelopes | structs de saída e de resultado | 4 | 3 | 1 | — |
| S — superfície CLI/MCP | declarações de flags, esquemas do registry, níveis | 4 | 3 | — | — |
| G — bugs de campo | IDs/leitor do grafo, reason do verify, strings do egm, base do stack | 3 | 3 | — | — |
| B — bus | bus, busrun, bind do `mcp.go`, versão | 1 | 5 | 1 | 3 |
| K — config | `internal/config`, init | 1 | 2 | — | — |
| D — docs | README, arquitetura, ADRs, árvore do cli.md, site | 3 | 4 | 1 | — |
| R — release e congelamento | goreleaser, release.yml, snapshot de contrato, CHANGELOG | 5 | 1 | — | — |

Ordem entre trilhas: C, G, B, K, D começam já. J e S começam já, mas se serializam nos arquivos comuns de `cmd/tb` como indicado. R3 e R5 fecham a fase.

## 5. Regras para quem implementa

- Um item (ou o grupo pequeno indicado numa trilha) por PR, branch `fix/`, `docs/` ou `chore/` a partir da `main`, Conventional Commits.
- Toda mudança de comportamento atualiza o cli.md (inglês + pt-BR) e o `[Unreleased]` do `CHANGELOG.md` no mesmo PR.
- Review em contexto novo, nunca pelo agente que implementou; veredito `PASS` do `tb stack` antes de pedir merge.
- Nunca editar `tests/hidden/` nem `.toolbelt/thresholds.yaml`. Nenhum fallback silencioso sem sandbox (C7 usa um ponto de injeção só de teste). O `tb` continua local: nenhuma chamada de rede é acrescentada.
- Uma correção de flake se prova com `go test -race -count=50` no pacote, rodado sozinho (`-p 1` em máquinas pequenas).

## 6. Decisões para o dono

Status: todas as recomendações O1–O19 aprovadas pelo dono em 2026-11-22; O20 aprovada em 2026-11-27.

| # | Decisão | Recomendação | Motivo |
|---|---------|--------------|--------|
| O1 | Remover `tb guard check --strict` (e o `strict` do MCP) antes da 1.0, ou congelá-lo como no-op depreciado? | remover | um no-op congelado na 1.0 precisa ser carregado por toda a 1.x; antes da 1.0 é o único momento barato |
| O2 | `tb_echo` / `tb_env`: remover, esconder ou manter? | remover os dois; não mover os campos do `tb_env` para o `tb_version` | ferramentas de debug numa lista congelada; o `tb_version` já dá os/arch/go |
| O3 | Nivelamento de §3.1 (bus, router, board, watchdog, egm, vault, red/blue/lab, bench, merge `prompt` experimentais) | aceitar | seus ADRs estão `proposed` ou seus dados de campo não assentaram; congelá-los agora travaria correções |
| O4 | Convenção de versão do JSON | `"v": "<kind>/<major>.<minor>"` em todo envelope de stdout/MCP; `version` inteiro só dentro de arquivos em disco | já usada por 7 envelopes, autodescritiva, e separa formato de arquivo de formato de transporte |
| O5 | `apply` do `tb_merge` (MCP) vs `--dry-run` (CLI): qual vence? | manter os dois como estão, documentados como a única exceção | um agente deve precisar optar por mover refs; quem usa a CLI espera `--dry-run` |
| O6 | Remover as 13 chaves de config mortas, ou documentá-las como reservadas? | remover, com o aviso de K2 | prometem limites (cpu/mem/time, allow_net) que o `tb` não aplica |
| O7 | `--no-check` (repetível) vs `--no-checks` (bool) vs `skip_checks` do MCP | renomear a flag da CLI para `--skip-check`, mantendo `--no-check` como alias oculto até a 2.0 | duas flags a uma letra de distância com tipos diferentes; o nome MCP já diz `skip` |
| O8 | Dependências de tarefa no bus (`deps` no `task.assign`): na 1.0? | adiar para a 1.1; entregar B2 (docs + aviso) na 1.0 | é uma regra nova do broker que interage com leases; o `tb router` já ordena pelas `dependencies` do ticket |
| O9 | `outcome` do claim no `tb_bus_status` (B6): na 1.0? | sim | pequeno, só leitura, e define o que `released` significa antes de o envelope ganhar `v` |
| O10 | Lista de tarefas do canal com estado (`tb bus tasks`): na 1.0? | adiar para a 1.1 | comando novo, aditivo depois sem quebrar nada; `tb bus log` cobre enquanto isso |
| O11 | Reservar paths sem tarefa / liberar um claim à mão: na 1.0? | adiar para a 1.1 | operações novas do broker que precisam de spec própria (leases, revogação); nenhum contrato piora com a espera |
| O12 | Checagem de sobreposição de paths entre tickets: na 1.0? | adiar para a 1.1 | uma checagem nova com política de falso positivo a desenhar; o `tb drift` já aponta edições fora do escopo depois do fato |
| O13 | KPIs de benchmark não implementados: manter ou tirar do benchmark.md? | mover para uma seção "Não implementado"; tirar a afirmação de significância | a documentação deve afirmar só o que existe (E7); a lista fica como roadmap |
| O14 | Status dos ADRs: aceitar 010 e 018; manter 009, 013, 016, 017 `proposed` | aceitar | o status passa a bater com o nível: accepted = stable, proposed = experimental |
| O15 | A invariante "No-CGO binary" vs a release com cgo do ADR-018 | editar a invariante para a regra entregue (R2) | a invariante é falsa hoje e o `tb mem check` trata invariantes como instruções de prioridade máxima |
| O16 | Mudanças de código de saída de §3.5 (erros de parse 1 → 2, timeout do sandbox 0 → 124, repasse com `--json` no sandbox) | aceitar | 2 para uso já é a regra do verify/egm/vault; 0 num filho que estourou o tempo é bug |
| O17 | Kinds `v` existentes em `0.x` (`bus/0.1`, …): subir para `1.0` no congelamento? | manter os números | versionam esquemas, não releases; mensagens guardadas no `bus.db` continuam batendo |
| O18 | `tb stack` sobre mudanças não commitadas (dogfood #13): na 1.0? | adiar para a 1.1; documentar que o `stack` compara commits | é um modo de entrada novo, não uma correção |
| O19 | Duração do período de uso do rc (E8) | 7 dias, pelo menos um rc | o dono é o principal usuário; uma semana cobre um ciclo completo de trabalho neste repositório |
| O20 | Base padrão do `tb stack`: manter `HEAD~1` ou usar o merge-base com a branch padrão (G6)? | merge-base, com `HEAD~1` como último recurso; sai como `v1.0.0-rc.5` | com `HEAD~1` uma branch com vários commits passa no gate só pelo último commit; depois da 1.0 mudar o padrão de uma flag é breaking. Além disso, o `main` recebeu um commit que não é só docs (#189) depois do rc.4, então o §7.5 já exigia um rc novo |

## 7. Passos da release

Siga o [release.md](../release.md); os acréscimos para a 1.0:

1. Fechar todo blocker; o dono resolve §6 e marca nesta spec os itens `should` adiados.
2. R1 mergeado (prereleases não são `latest`). R5 move `[Unreleased]` para `[1.0.0-rc.1]` nos dois changelogs.
3. Criar a tag `v1.0.0-rc.1` numa `main` verde; o workflow de release monta o draft, roda `smoke` e `floors` e o publica como prerelease.
4. Instalar como o release.md descreve (sha256 + certificado do bundle) em `~/go/bin`; usar neste repositório. Achados novos vão para o [dogfood](../campo/dogfood.md). Um achado `high`: corrigir na `main`, criar a tag `v1.0.0-rc.2` e recomeçar o período de uso.
5. Quando E1–E9 valerem: renomear `[1.0.0-rc.N]` para `[1.0.0]` (mantendo o texto das entradas do rc) e criar a tag `v1.0.0` no mesmo commit do último rc ou num descendente só de documentação.
6. Depois da 1.0: a `main` recebe só correções e mudanças aditivas; uma mudança incompatível abre uma spec de 2.0. Nunca reutilizar uma tag.

## 8. Correções das auditorias

As auditorias foram somente leitura e em parte erradas; quem implementa se apoia nesta seção, não no texto das auditorias.

- `TestMarkerOnlyOnHead` não é flake: é um fixture que os testes de `internal/verify` e `internal/stack` escrevem de propósito para fazer o verify falhar no head; seu `--- FAIL` é logado em nível debug dentro de testes que passam (C8).
- `router.bus.claim_timeout` é lida (`internal/router/bus.go:111`); não está morta.
- "threshold vs `--min-coverage`" no MCP não é divergência: `threshold` (visual) e `min_coverage` existem dos dois lados. `class` mapeia um argumento posicional, pela regra de §3.3.
- As 54 ferramentas MCP do README/CLAUDE.md estão certas: 50 registros literais mais `tb_red_search`, `tb_red_load`, `tb_blue_search`, `tb_blue_load`, montadas em `internal/registry/secplay.go`.
- `readSource` (symlinks e FIFOs) está em `internal/graph/index_common.go`, não no vault; a Q5 da spec do vault o registra para a varredura do grafo.
- `expiresAt` (`internal/egm/vendor.go:178`) lê o arquivo de credencial do fornecedor; não é campo do tb e continua em camelCase.
- As falhas de lint na amostra da auditoria (`filepathJoin` do gocritic em `busrun/routertask_test.go`, `secplay`) vieram de branches de PR; a `main` está limpa de lint hoje.
- Os problemas de `sha256sum` no macOS e do `\` inicial no Windows já são tratados em `scripts/ci-smoke-model.sh` e `release.yml`.
