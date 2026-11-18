[🇺🇸 English](../../campo/crush.md) | 🇧🇷 Português

# Campo: charmbracelet/crush

Estudo de campo do `tb` (toolbelt @ `c8aae04`) usando o repositório público third-party **charmbracelet/crush** apenas em clone local (`/tmp/target/crush`). Sem push, issues, PRs ou comentários no upstream.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [charmbracelet/crush](https://github.com/charmbracelet/crush) |
| Linguagem | Go (`go 1.27.4` no `go.mod`) |
| Tamanho (clone shallow) | ~25 MiB disco; **904** arquivos `.go`; ~**204 480** LOC Go |
| Instrumentação AI existente | `AGENTS.md` (raiz); `internal/ui/AGENTS.md`; `internal/cmd/stats/AGENTS.md`; `internal/oauth/callback/AGENTS.md`; `.agents/skills/` (`builtin-skills`, `shell-builtins`); `crush.json`; schema/docs de skills/hooks |
| Toolchain | **Parcialmente configurada** |

Toolchain neste VM:

- `GOTOOLCHAIN=go1.27.4` baixou e compilou o binário (`go build .` OK, ~1 m26 s na primeira vez).
- `go test ./internal/filepathext` OK; `go test ./internal/skills` OK.
- `task` instalado via `go install`; `task lint:log` OK após o fix.
- **Não instalados**: `golangci-lint`, `gofumpt` (lint completo `task lint` / `task fmt` não rodados).
- Suite completa `go test ./...` / `task test` **não** foi executada end-to-end (custo alto; fora do escopo mínimo da tarefa escolhida).

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Fechar buraco no `scripts/check_log_capitalization.sh`: o grep só pegava `slog.Info/Error/...("msg")` e **não** `slog.*Context(ctx, "msg")`, apesar de `AGENTS.md` exigir capitalização e `task lint:log` depender desse script. |
| Por quê | Lacuna real e pequena, com repro local e auto-teste; alinhada à instrumentação AI do repo; maintainer-plausível (fortalece lint já documentado). |
| Issue | Sem issue dedicada; motivada por `AGENTS.md` § Build/Test/Lint + script existente. Issues lidas (#3366, #3389, #254, etc.) eram grandes demais ou Windows-específicas. |
| Completada? | **Sim**, no clone local (branch `campo-local-log-lint`, commit `1f2974c`). **Não** enviada ao upstream (proibido). |
| Checks nativos | `./scripts/check_log_capitalization.sh` → exit 0; `./scripts/check_log_capitalization_test.sh` → OK; `task lint:log` → exit 0; `go build .` → exit 0. `golangci-lint` / `task test` não rodados. |

Detalhe da implementação: regex em duas alternativas — clássica `slog.(Error\|Info\|Warn\|Debug\|…)\(["'][a-z]` e Context `slog.(Error\|Info\|Warn\|Debug)Context\([^,]+,[[:space:]]*["'][a-z]`. Primeira tentativa (só acrescentar `*Context` na lista de nomes) **falhou** porque a mensagem não é o 1º argumento; segunda tentativa gerou falsos positivos; terceira passou o self-test.

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (make build) | OK; `toolbelt c8aae04` | neutro | `2026-11-15T20:47:28Z`; commit/built preenchidos via ldflags |
| `tb version` (sem ldflags) | `0.1.0-dev`, commit/built `unknown` | atrapalhou | `/tmp/tb-nolflags version` |
| `tb init` | criou `.toolbelt/` + `config.yaml` mode **0600** | ajudou (bootstrap) / atrapalhou (perms) | `stat` → `600 .toolbelt/config.yaml` |
| `tb mem init` | `invariants.yaml` 0600 + 3 regras anti-cheat | ajudou | inclui `NUNCA modifique arquivos em tests/hidden/` |
| `tb mem add <regra>` | OK posicional | ajudou | `inv-…` adicionada |
| `tb mem add --type/--title` (docs) | **unknown flag: --type** | atrapalhou | divergência docs/cli.md vs CLI |
| `tb mem list` / `check` | OK; retenção 1.00 | neutro | 4 invariantes no fim |
| `tb graph --enable index .` | 5136 nós, 38724 arestas, ~0,21 s | ajudou | indexou Go + markdown (AGENTS.md) |
| `tb graph stats/search` | OK; achou scripts e seções AGENTS.md | ajudou | search `AGENTS`, `check_log` |
| `tb graph impact` sem `--file` | exit 1 `--file é obrigatório` | atrapalhou | UX sem default no diff atual |
| `tb graph impact --file scripts/check_log_capitalization.sh` | `(vazio)` | neutro | shell script sem arestas úteis |
| `tb graph deps --file …` | `(vazio)` | neutro | idem |
| Ticket/`rd.md` + Scope | YAML frontmatter funciona; `## Scope` bullets **não** parseados | atrapalhou (UX) | `scope-missing` no review com markdown headings |
| `tb drift` (YAML scope literal) | OK, sem out_of_scope | ajudou | `rd.md` frontmatter |
| `tb drift` com `scripts/*.sh` (YAML) | OK (glob via `filepath.Match`) | ajudou | globs **funcionam** no matcher |
| `tb review` | findings vazios com ticket YAML correto | ajudou | fresh_context true, 3 goldfish |
| `tb review` em commit `tests/hidden/` | critical em invariante hidden | atrapalhou (FP no caso “criar hidden”) | exit 1 + guard protected_path |
| `tb guard check` no diff do fix | violations null | ajudou | JSON limpo |
| `tb guard` + `.toolbelt/guard.yaml` custom | **ignorado** | atrapalhou | protegemos `scripts/check_log_capitalization_test.sh` no yaml; alteração não flagada |
| `tb guard` em `.toolbelt/thresholds.yaml` | protected_path error (hardcoded) | ajudou (anti-cheat) / atrapalhou (inflexível) | |
| `tb verify run` (test_cmd=self-test) | PASS, 1 iter, ~0,03 s | ajudou | visible_passed true; hidden_available false |
| `tb verify` após touch thresholds | FAIL imediato; **testes não rodaram** | atrapalhou | iterations=0 |
| `tb verify --ruler visual` (sem manifesto) | PASS, `skipped: true` | neutro | skip_reason manifesto ausente |
| `tb verify --ruler visual` (manifesto sem refs) | PASS, **`skipped: false`**, screen `skipped: true` | atrapalhou | falso “não skipped” no relatório raiz |
| `verify.build_cmd` / `lint_cmd` no config | **não consumidos** pelo `tb verify` | atrapalhou | só `TestCmd`/`HiddenTestCmd` no código |
| `tb bench run` sem `--agent` com `default_agent` setado | exit 1 obrigatório `--agent` ou `--fake` | atrapalhou | default_agent ignorado |
| `tb bench run --fake` | OK 33,3% nas fixtures do toolbelt | ajudou | **não** exigiu `--fake-config` |
| `tb bench compare --fake` A/B | OK | ajudou | |
| `tb bench compare --agent …` | unknown flag `--agent` | atrapalhou | confirmado |
| `tb mcp` tools/list + tools/call | OK; `tb_version` respondeu | ajudou | stdio JSON-RPC |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `Error: unknown flag: --agent`; help sem a flag |
| `guard.yaml` ignored | **confirmado** | yaml protegia o test script; `guard check` no diff do fix → `violations: null`; só paths hardcoded em `internal/guard/guard.go` |
| outdated CLI docs | **confirmado** | `docs/cli.md` mostra `tb mem add --type/--title`; CLI real é `tb mem add <regra>`; README ainda diz “Fase 0” enquanto roadmap lista Fase 2 |
| version shows `unknown` | **confirmado (parcial)** | sem ldflags: version `0.1.0-dev`, **commit/built = unknown**; com `make build`: ok |
| config files created 0600 | **confirmado** | `config.yaml` e `invariants.yaml` → mode 600 |
| visual ruler reports `skipped=false` when skipping | **confirmado** | com `screens.yaml` e ref ausente: top-level `"skipped": false` mas screen `"skipped": true` |
| `default_agent` ignored | **confirmado** | config `default_agent: echo AGENT-RAN {prompt}` + `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | diff em `.toolbelt/thresholds.yaml` → verify FAIL, iterations 0, testes não executados |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake` sem `--fake-config` concluiu com sucesso; flag só altera comando se fornecida |
| `mem add` positional | **confirmado** | usage `tb mem add <regra>`; `--type` unknown |
| Drift/Scope need literal paths (no globs) | **refutado (matcher); confirmado (UX markdown)** | YAML `scripts/*.sh` casa via `filepath.Match`; bullets sob `## Scope` **não** são lidos (só `Scope:` same-line ou frontmatter YAML) |
| `graph impact` needs `--file` | **confirmado** | `--file é obrigatório` |
| review false positives on the tests/hidden invariant | **confirmado** | commit só com `tests/hidden/ok.go` → finding critical + exit 1 |
| verify runs only partial checks and misses build errors | **confirmado** | config aceita mentalmente `build_cmd`/`lint_cmd` mas `cmd/tb/verify.go` só aplica `TestCmd`/`HiddenTestCmd`; build quebrado não seria exercido pelo verify padrão |
| tb does not push agent toward completing the whole task | **confirmado (neste estudo)** | 1ª “correção” do regex não detectava Context (self-test falhou / context_exit=0); tb não bloqueou o fluxo até eu rodar o self-test de propósito; review/drift não cobram acceptance checkboxes |

## Problemas novos

### P1 — Parser de Scope ignora listas markdown sob heading

- **Repro**: `rd.md` com `## Scope` + bullets `- path`; `tb review` / `tb drift`.
- **Expected**: scope preenchido a partir da lista.
- **Actual**: `scope-missing` / scope vazio; drift não marca out_of_scope (porque `len(scope)==0` desativa a checagem).

### P1 — Relatório visual: `skipped=false` com todas as telas skipped

- **Repro**: `.toolbelt/screens.yaml` sem `references`; `tb verify run --ruler visual --skip-unavailable --json`.
- **Expected**: `skipped: true` no relatório raiz (ou verdict distinto).
- **Actual**: `"skipped": false`, `"verdict": "PASS"`, screens com `skipped: true`.

### P2 — Primeira iteração do lint Context fácil de “passar” no verify se o test_cmd for fraco

- **Repro**: regex que só adiciona `*Context` ao grupo de nomes; `test_cmd` que só roda o script no tree limpo.
- **Expected**: verify exigir regressão que falhe com Context lowercase.
- **Actual**: tree limpo passa; buraco só aparece com fixture negativa (por isso o self-test foi essencial — tb não o exige).

### P2 — `graph impact`/`deps` inúteis para shell scripts

- **Repro**: indexar crush; `impact --file scripts/check_log_capitalization.sh`.
- **Expected**: pelo menos arestas doc/mention úteis ou mensagem clara “sem nós”.
- **Actual**: `(vazio)` sem diagnóstico.

### P3 — `guard.yaml` criado pelo usuário fica 0644 enquanto configs do tb são 0600

- **Repro**: `cat > .toolbelt/guard.yaml`; comparar com `config.yaml`.
- **Expected**: política de permissões documentada / uniforme.
- **Actual**: mistura 0600 (tb) e 0644 (manual); yaml ainda assim ignorado.

### P3 — MCP `tb_graph_*` listados sem deixar claro na tools/list que o grafo está off por padrão

- Tools aparecem; CLI exige `--enable`/`TB_GRAPH=on`. Não exercitado call com grafo off além da listagem.

## O que o tb pegou e o que deixou passar

**Pegou**

- Guard em path hardcoded `.toolbelt/thresholds.yaml` e `tests/hidden/*`.
- Drift/review de scope quando o ticket YAML está correto.
- Verify PASS no self-test do lint após o fix.
- Graph indexou `AGENTS.md` e localizou os scripts tocados.
- Visual skip quando não há manifesto (`skipped: true`).

**Deixou passar / não cobriu**

- Não leu `AGENTS.md` na hora do `init`/onboarding para sugerir `test_cmd`/`lint_cmd` reais (`task test`, `task lint:log`).
- Não executou `build_cmd`/`lint_cmd` mesmo presentes no yaml editado.
- Não impediu a 1ª correção incompleta do regex (faltou fixture negativa obrigatória).
- `guard.yaml` custom ignorado → falsa sensação de política configurável.
- Impacto de mudança em `.sh` vazio.
- Relatório visual raiz mentiu `skipped=false` com skips por tela.
- Não há `tb ticket` (Fase 3): “tickets with Scope” = arquivo `rd.md` manual.

Comparando com nativo: o self-test shell e `task lint:log` foram a fonte de verdade; o tb verify só foi tão bom quanto o `test_cmd` que eu configurei.

## Interação com instrumentação existente / onboarding

| Aspecto | Observação |
|---------|------------|
| `tb init` | Cria `.toolbelt/` paralelo; **não** importa `AGENTS.md`, `.agents/skills`, `Taskfile.yaml` nem `crush.json`. |
| Duplicação | Invariantes genéricas do `mem init` vs regras já em `AGENTS.md` (capitalização de logs, CGO, testes). Conteúdo não sincronizado. |
| Conflito | Invariante `tests/hidden/` do tb vs fluxo natural de adicionar suíte oculta no próprio harness do tb — review/guard punem o que o produto pede. |
| Ignore | Regras ricas do crush (gofumpt, `task lint:log`, golden `-update`, mock providers) ficam só no AGENTS; agent com tb não é empurrado a lê-las. |
| Graph | **Lê indiretamente**: indexa `AGENTS.md` e AGENTS aninhados como docs (seções/headings). Útil para search, não para onboarding normativo. |
| O que o tb deveria importar | (1) comandos Build/Test/Lint do AGENTS/Taskfile → `verify.test_cmd` / lint; (2) proibições explícitas (“NUNCA…”, file perms octal, capitalização) → `invariants.yaml`; (3) paths de context files já listados pelo crush (`AGENTS.md`, `CRUSH.md`, …) → `mem`/`ctx`; (4) não sobrescrever skills `.agents/` — referenciar. |

## Sugestões de melhoria

1. **[P0][verify]** Honrar `build_cmd` e `lint_cmd` (ou documentar que não existem) e falhar se o config declarar chaves ignoradas.  
2. **[P0][docs]** Alinhar `docs/cli.md` / README (`mem add`, fase do projeto, guard.yaml) com a CLI real.  
3. **[P0][guard]** Carregar `.toolbelt/guard.yaml` ou remover a menção da docs; hoje é armadilha.  
4. **[P1][review/drift]** Parsear listas markdown sob `## Scope` / `## Escopo`; avisar quando scope está vazio (em vez de desligar drift).  
5. **[P1][verify/visual]** Propagar `skipped=true` no relatório raiz se todas as screens foram skipped.  
6. **[P1][bench]** Respeitar `bench.default_agent`; adicionar `--agent` em `bench compare` ou documentar só `--fake`/JSONL.  
7. **[P1][init/mem]** Onboarding: detectar `AGENTS.md` / Taskfile e pré-preencher verify + invariantes (grupo “já tem instrumentação AI”).  
8. **[P2][graph]** Mensagem explícita quando impact/deps estão vazios; opcional indexação de scripts como “task” ligados a docs que os citam.  
9. **[P2][verify]** Exigir ou gerar fixture negativa para rulers de lint (evita “PASS” com buraco residual).  
10. **[P2][version]** `go build` sem ldflags não deveria reportar commit `unknown` sem aviso; ou embed via `debug.ReadBuildInfo`.  
11. **[P3][config]** Revisar 0600 vs colaboração em `.toolbelt/` (gitignored?).

## Métricas

| Métrica | Valor |
|---------|-------|
| Wall time (sessão instrumentada tb, evidence log) | ~**3 min** (`20:47:28Z` → `20:50:33Z`), excluindo download inicial Go 1.27 / build crush (~1 m26 s antes) |
| Invocações tb nomeadas no log | **18** CMD tags + várias ad-hoc (init/mem/bench/mcp/version) ≈ **~30+** |
| Falhas tb observadas (exit 1) | **10** ocorrências no evidence (inclui probes esperados: impact sem file, compare --agent, thresholds, review-hidden, etc.) |
| Crashes tb | **0** |
| MCP | tools/list + `tb_version` OK |
| Alterações no repo toolbelt | **somente** este arquivo `docs/campo/crush.md` |
| Alterações no crush | apenas locais em `/tmp/target/crush` (não push) |
