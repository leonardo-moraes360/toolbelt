🇧🇷 Português | [🇺🇸 English](../en/campo/ollama.md)

# Estudo de campo: ollama/ollama

Estudo realizado em 2026-11-15 com `tb` buildado deste repo (`make build` → `toolbelt c8aae04`). Clone local em `/tmp/target/ollama` (shallow). **Nenhuma interação** com o upstream (sem push, issue, PR ou comentário).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [ollama/ollama](https://github.com/ollama/ollama) |
| Linguagem | Go (~1097 `.go`), UI TS (~50), C/C++ nativo (llama/MLX) |
| Tamanho | ~80 MiB shallow clone; ~1657 arquivos (excl. `.git`); Go 1.26.10 no `go.mod` |
| Instrumentação AI existente | `AGENTS.md` (build: cmake + `go build`/`go run`); `CLAUDE.md` (apenas aponta para `AGENTS.md`); docs de integrações Claude/Copilot em `docs/integrations/` (produto, não harness de agentes) |
| Toolchain | **Parcial.** `GOTOOLCHAIN=auto` baixou Go 1.26.10; `go test ./api/` e `go test ./cmd/` OK. CMake configurado falhou: linker sem `libstdc++` (`cannot find -lstdc++`); `ninja` ausente. Build nativo completo (`cmake -B build && cmake --build`) **não** foi possível. `golangci-lint` ausente. GPU backends não tentados. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Evitar falha `flag accessed but not defined: verbose` ao rodar modelo via `ollama launch` (contexto cobra sem a flag). Helpers `verboseEnabled` / `ensureVerboseFlag`; `/set verbose` e `/set quiet` registram a flag sob demanda; 3 testes unitários. |
| Por quê | Bug real aberto, reproduzível, escopo pequeno em pacote Go puro com testes; maintainer já viu PR similar não mergeado. |
| Issue | https://github.com/ollama/ollama/issues/14654 (PR relacionado não mergeado: https://github.com/ollama/ollama/pull/14701) |
| Completada? | **Sim, localmente** no commit `a36cdca` (apenas clone local). Não enviada ao upstream. |
| Native test/lint/build | `go test ./cmd/ -count=1` → **ok**. Lint do projeto: não há script local + sem golangci. Build nativo cmake: **falhou** (libstdc++/ninja). `go build` do binário principal não exigido para a correção; pacote `cmd` compila nos testes. |

Arquivos alterados no clone: `cmd/cmd.go`, `cmd/interactive.go`, `cmd/cmd_test.go`. Ticket `rd.md` com frontmatter YAML (`scope`, `base_commit`, `acceptance`).

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK (`c8aae04` com `make build`; `0.1.0-dev` + `commit: unknown` sem ldflags) | neutro | 0,005 s; `go build` sem ldflags → unknown |
| `tb init` | OK; criou `.toolbelt/` | ajudou | config + vault + logs + bench; **não** leu `AGENTS.md` |
| `tb mem init/list/add/check` | OK | ajudou | 5 invariantes; `mem add <regra>` posicional funciona; `--type` da docs falha |
| Ticket/`rd.md` + Scope | OK após frontmatter YAML | ajudou / atrapalhou | Markdown `## Scope` não basta; precisa YAML como em `LoadTicket` |
| `tb graph --enable index cmd/` | OK (1138 nós) | ajudou | `index --path` (docs) é flag desconhecida; positional funciona |
| `tb graph rebuild` | OK (8924 nós, ~1,05 s) | ajudou | repo grande indexável |
| `tb graph search/impact` | OK com `--json` / após index | neutro / atrapalhou | `impact` exige `--file`; texto às vezes `(vazio)` antes do rebuild; search não acha símbolo até reindex |
| `tb graph callers --file` | Erro | atrapalhou | docs pedem `--file`; CLI rejeita (`unknown flag: --file`) |
| `tb verify run` (test_cmd=`go test ./cmd/`) | PASS | ajudou | 1,2 s; `hidden_available: false` ainda PASS |
| `tb verify --ruler visual` | PASS + skip | neutro | JSON `skipped: true`, reason manifesto ausente; Chrome presente |
| `tb drift` | OK no escopo literal | ajudou | 0,01 s; `cmd/**` também funciona |
| `tb review` | OK com warnings | atrapalhou | 3 warnings de completeness mesmo com testes no diff (aceite “pouco evidenciado”) |
| `tb guard check` | OK hardcoded | ajudou / atrapalhou | detecta `tests/hidden` e `.toolbelt/thresholds.yaml`; **ignora** `.toolbelt/guard.yaml` custom |
| `tb bench run --fake` | OK (9 tasks, 33,3%) | ajudou | usou tasks do próprio toolbelt; ollama sem fixtures |
| `tb bench compare` A/B `--fake` | OK | ajudou | graph-off vs graph-on |
| `tb bench compare --agent` | Erro `unknown flag` | atrapalhou | confirmado |
| `tb mcp` + `tools/list` | OK | ajudou | tools: version, echo, env, run, graph_* |
| Checks nativos vs tb | `go test ./cmd/` OK | — | tb verify alinhado quando `test_cmd` aponta ao pacote certo; com `test_cmd=./format/` e `cmd` quebrado → PASS falso |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `--help` sem flag; `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmado** | manifesto com `cmd/cmd.go` protegido → `violations: null` no diff que altera esse arquivo; `NewChecker` usa paths hardcoded |
| outdated CLI docs | **confirmado** | `docs/cli.md`: `tb serve`, `tb mem add --type`, `graph index --path`, `graph callers --file` ≠ CLI real (`tb mcp`, add posicional, index positional, callers sem `--file`) |
| version shows `unknown` | **confirmado (parcial)** | sem ldflags: `commit: unknown`, `built: unknown`; com `make build`: hash do commit |
| config files created 0600 | **confirmado** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → `600` |
| visual ruler reports `skipped=false` when skipping | **refutado** | JSON: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`; texto: “Skip gracioso…” |
| `default_agent` ignored | **confirmado** | config `default_agent: echo-agent {prompt}` mas `tb bench run` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | commit com `.toolbelt/thresholds.yaml` → verify `FAIL`, `iterations: 0`, violation protected_path |
| default fake agent needs `--fake-config` | **refutado (neste uso)** | `tb bench run --fake` sem `--fake-config` concluiu 9 tarefas |
| `mem add` positional | **confirmado** | CLI: `tb mem add <regra>`; docs ainda mostram flags `--type/--title` |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `cmd/**`, `cmd/`, `cmd/*` OK; glob recursivo genérico `**/*_test.go` só casa 1 nível (Go `filepath.Match`) |
| `graph impact` needs `--file` | **confirmado** | sem flag: `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | **não reproduzido neste diff** | `--standards invariants,scope` → `findings: []`; false positives observados em **completeness**/acceptance, não em hidden |
| verify runs only partial checks and misses build errors | **confirmado (condicional)** | com `test_cmd=go test ./format/` e syntax error em `cmd/cmd.go` → verify **PASS**; com `./cmd/` o mesmo erro → **ESCALATE** |
| tb does not push agent toward completing the whole task | **confirmado (observacional)** | tb não apontou `AGENTS.md`/cmake, não cobrou build nativo, não sugeriu registrar flag em `LaunchCmd`; review pediu aceite “no diff” em vez de guiar fechamento; workflow inteiro foi conduzido manualmente |

## Problemas novos

1. **P1 — `tb guard check --strict --json` não falha o exit code**  
   - Repro: diff que toca `tests/hidden/*`; `tb guard check BASE..HEAD --strict` → exit 1; idem com `--json` → exit **0** (JSON ainda lista violations).  
   - Esperado: `--strict` falhar independentemente de `--json`.  
   - Atual: early `return` no ramo JSON antes de avaliar `--strict` (`cmd/tb/guard.go`).

2. **P2 — Saída texto de `graph impact`/`search` pode mostrar `(vazio)` com grafo populado (ou pré-rebuild)**  
   - Repro: após só `index cmd/`, `impact --file cmd/cmd.go` em texto → `(vazio)`; com `--json` ou após `rebuild` há nós.  
   - Esperado: mesma informação útil em texto e JSON.  

3. **P2 — `graph search` não reflete código novo até reindex; path no resultado pode perder diretório**  
   - Repro: após fix, sem reindex → search `verboseEnabled` vazio; após `index cmd/` acha `func:cmd.verboseEnabled` com path `cmd.go` (não `cmd/cmd.go`).  
   - Esperado: path canônico e orientação a reindexar após mudanças.

4. **P2 — Docs vs CLI em vários pontos além dos já listados**  
   - `tb graph callers` docs com `--file`; help real só `--function`/`--depth`.  
   - `tb serve` documentado; binário expõe `tb mcp`.

5. **P3 — `tb verify` PASS com `hidden_available: false` e `hidden_passed: false`**  
   - Repro: config sem suite oculta real → verdict PASS.  
   - Esperado: aviso explícito ou verdict degradado quando hidden não está disponível.

6. **P3 — `guard.yaml` criado pelo usuário fica 0644 enquanto configs do `tb` ficam 0600**  
   - Inconsistência de permissões; manifesto ainda assim é ignorado (P0 conhecido).

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo quando o ticket aponta paths errados (`format/**` vs mudanças em `cmd/`).
- Paths hardcoded protegidos (`tests/hidden/*`, `.toolbelt/thresholds.yaml`) e abort de verify nesses casos.
- Falha de compile/test quando `test_cmd` cobre o pacote quebrado.
- Indexação de grafo em escala (8k+ nós) e `impact`/`deps` úteis após rebuild.
- MCP `tools/list` operacional.

**Deixou passar / enfraqueceu**
- `guard.yaml` custom (proteger `cmd/cmd.go`) completamente ignorado.
- Syntax error em `cmd` com verify apontando só para `./format/` → PASS.
- Review completeness: marcou critérios de aceite como “pouco evidenciados” apesar de helpers + testes no diff (ruído).
- Não leu `AGENTS.md` (cmake/libstdc++/ninja) — agente sem tb também precisaria ler, mas tb não importa nem resume isso no onboarding.
- Não há ruler de lint/build nativo separado; só o que estiver em `test_cmd`.
- `bench compare` sem `--agent` impede A/B com agente real só via compare.

## Interação com instrumentação existente / onboarding

- **`AGENTS.md` / `CLAUDE.md`**: tb **não lê, não importa, não conflita e não espelha**. Nenhum referência em `cmd/` ou `internal/` do toolbelt. `tb init` só cria `.toolbelt/{config,vault,logs,bench}` + depois `mem init` → `invariants.yaml`.
- **Duplicação**: invariantes anti-cheat do tb (`tests/hidden`, thresholds) são genéricas e **não** cobrem regras de build do ollama (cmake paralelo, `go run . serve`, CGO).
- **Onboarding**: não gera progress log / feature list estilo harness; não cria ticket a partir de issues; não sugere `verify.test_cmd` a partir de `AGENTS.md` ou Makefile.
- **O que o tb deveria importar**: conteúdo de `AGENTS.md`/`CLAUDE.md` (e links para `docs/development.md`) como fatos/mandate de build; detectar `go test`/`cmake` commands; opcionalmente mapear labels de issue → ticket com Scope.

## Sugestões de melhoria

1. **[P0][guard]** Honrar `.toolbelt/guard.yaml` (merge com defaults hardcoded) — área `guard`.  
2. **[P0][docs]** Alinhar `docs/cli.md` / README ao CLI real (`mcp`, `mem add`, flags de graph) — área `docs`.  
3. **[P1][guard]** Aplicar `--strict` também no ramo `--json` — área `guard` CLI.  
4. **[P1][bench]** Aceitar `--agent` em `bench compare` (ou herdar `default_agent`) — área `bench`.  
5. **[P1][bench]** Respeitar `bench.default_agent` quando `--agent`/`--fake` omitidos — área `bench`/`config`.  
6. **[P1][verify]** Modo “repo checks”: além de `test_cmd`, opcional `build_cmd`/`lint_cmd`; falhar ou warn se hidden indisponível — área `verify`.  
7. **[P1][mem/ctx]** `tb init` / `tb ctx`: importar `AGENTS.md`/`CLAUDE.md` para vault + sugerir `verify.*` — área `mem`/`init`.  
8. **[P2][review]** Completeness: considerar nomes de teste/símbolos no diff como evidência de aceite — área `review`.  
9. **[P2][graph]** UX: mensagem “grafo vazio — rode index”; paths canônicos; callers com `--file` ou docs corrigidas — área `graph`/`docs`.  
10. **[P2][config]** Permissões 0640/0644 para configs não secretas; documentar 0600 — área `config`.  
11. **[P2][router/ux]** Checklist de tarefa (build nativo, lint, aceite) para empurrar fechamento completo — área `onboarding`/`ticket`.  

## Métricas

| métrica | valor |
|---------|-------|
| Wall time do estudo | ~237 s de instrumento + ~4 min de testes Go/deps na primeira corrida (ordem de **~6–10 min** total de sessão de campo) |
| Invocações `tb` registradas | **59** blocos de comando no log de evidência |
| Falhas tb (exit ≠ 0) | **13** (maioria probes intencionais: flags inexistentes, drift forçado, guard strict, compare --agent, graph desabilitado, etc.) |
| Soma de tempos tb medidos | ~**9,7 s** (exclui `go test` nativo e download de módulos) |
| Commits locais no clone ollama | feature `a36cdca` + commits descartados de repro de guard/verify |
| Artefato neste repo | apenas este arquivo em `docs/campo/ollama.md` |
