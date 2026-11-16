# Campo rodada 2: charmbracelet/crush

Estudo de campo ROUND 2 do `tb` (toolbelt @ `a46c295`) no repositório third-party **charmbracelet/crush**, grupo **with existing AI-agent instrumentation**. Clone local apenas (`/tmp/target/crush`). Sem push, issues, PRs ou interação com o upstream.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [charmbracelet/crush](https://github.com/charmbracelet/crush) |
| Linguagem | Go (`go 1.27.4` no `go.mod`) |
| Tamanho (clone shallow) | ~25 MiB disco; **904** arquivos `.go`; ~**204 515** LOC Go |
| Instrumentação AI existente | `AGENTS.md` (raiz); `internal/ui/AGENTS.md`; `internal/cmd/stats/AGENTS.md`; `internal/oauth/callback/AGENTS.md`; `.agents/skills/` (`builtin-skills`, `shell-builtins`); `crush.json`; schema/docs de skills/hooks |
| Toolchain | **Parcialmente configurada** |

Toolchain neste VM:

- `GOTOOLCHAIN=go1.27.4`: `go build .` OK (~1 m16 s); `go test ./internal/stringext/` OK; `task` via `go install`; `task lint:log` OK.
- **Não instalados**: `golangci-lint`, `gofumpt` (`task lint` / `task fmt` completos não rodados).
- Suite completa `go test ./...` / `task test` **não** executada (custo; fora do escopo da tarefa).

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `stringext.Capitalize`: trocar `cases.Title` (title-case de cada palavra, que transforma `"API"` → `"Api"`) por uppercase só da primeira rune, preservando o resto (acrônimos e casing interno). Incluir testes unitários. |
| Por quê | Bug real e pequeno, com repro local (`Title("API")=="Api"`); call sites usam Capitalize em títulos de erro e labels de UI (`internal/agent`, `internal/ui/...`). Maintainer-plausível. |
| Issue | Sem issue dedicada; motivada por inspeção de `internal/stringext` + call sites. Issues abertas grandes (#4193 timeout, #4242 TMPDIR, etc.) ficaram de fora. |
| Completada? | **Sim**, no clone local (branch `campo2-local-capitalize`, commit `ca9566c`). **Não** enviada ao upstream (proibido). |
| Checks nativos | `go test ./internal/stringext/` → PASS; `./scripts/check_log_capitalization.sh` → exit 0; `task lint:log` → exit 0; `go build .` → exit 0. `golangci-lint` / `task test` não rodados. |
| Rodada 1 (comparação) | Lint de capitalização `slog.*Context` em `scripts/check_log_capitalization.sh` — **tarefa diferente** desta rodada. |

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph --enable index /tmp/target` (basename=`target`) → **5183** nós / 39091 arestas (antes zerava). Índice em `/tmp/target/crush` → 5157 nós. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir: `hidden_test_cmd=false` + `test_cmd=true` → `hidden_available: true`, `hidden_passed: false`, verdict `ESCALATE`, exit 1. Com cmd real no fix → `hidden_passed: true`. Default init agora `hidden_test_cmd: ""`. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff tocando path protegido: `--json` exit 0; `--strict --json` exit **1** com JSON válido no stdout. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `internal/stringext/string_test.go` → `protected_path` no diff do fix. Hardcoded `.toolbelt/thresholds.yaml` e `tests/hidden/*` continuam. |
| 5 | init detecção de toolchain | **funcionou** | `go.mod` → `verify.test_cmd: go test ./...` (não default genérico errado). |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket só *mencionando* `tests/hidden/**` → sem finding critical de invariante. Commit real em `tests/hidden/ok.go` → `critical` + guard `protected_path`. |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Sem `--agent`, usou `default_agent: echo AGENT-RAN {prompt}` (JSONL com `AGENT-RAN …`). `compare --agent` existe no help; `compare --config-a/--config-b --agent 'echo …'` exit 0. |
| 8 | review `## Scope`/`## Escopo`, listas MD e globs `**` | **funcionou** | `## Escopo` + bullets + `internal/stringext/**/*.go` → drift limpo. Escopo só `string.go` → `out_of_scope: string_test.go`. `**/*_test.go` manteve o test in-scope e marcou `string.go` fora. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Verify PASS com `build_ran/passed` e `lint_ran/passed` true (`go test -c` + `check_log_capitalization.sh`). |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** (com ressalva) | Com `AGENTS.md`: gravou `.toolbelt/agent_docs.yaml` + hints; `--write-agents` **não** sobrescreveu. Em dir sem AGENTS: `--write-agents` semeou esqueleto. Ressalva: hints extraíram blocos Go grandes como “comandos”. |
| 11 | graph Python `calls` | **não se aplica** | Repo Go; arestas `calls` Go OK (29566). Extrator Python não exercitado aqui. |
| 12 | verify visual skipped agregado + docs CLI | **funcionou** | `screens.yaml` sem refs → `"skipped": true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` de `mem add` alinhado ao posicional (não mais `--type/--title`). |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` (make build) | OK `a46c295` | neutro | ldflags preenchidos |
| `tb init` | `.toolbelt/` + detect Go + lê AGENTS.md | **ajudou** | `agent_docs.yaml`; test_cmd `go test ./...` |
| `tb init --write-agents` (com AGENTS) | não sobrescreve | ajudou | `diff` AGENTS.md vazio |
| `tb init --write-agents` (sem AGENTS) | semeia mínimo | ajudou | `/tmp/noagents/AGENTS.md` criado |
| `tb mem init/add/list/check` | OK; retenção 1.00 | ajudou | 4 invariantes |
| `tb mem add --type` | unknown flag | neutro | docs CLI já corrigidos; flag antiga só atrapalha quem memorizou rodada 1 |
| `tb graph --enable index` | 5157–5183 nós | **ajudou** | SkipDirs raiz corrigido |
| `tb graph search Capitalize` | achou `func:stringext.Capitalize` | ajudou | |
| `tb graph impact --file string.go` | lista callers UI/agent | ajudou | |
| `tb graph impact` sem `--file` | exit 1 obrigatório | atrapalhou (UX) | igual rodada 1 |
| `tb graph callers --function Capitalize` | 5 callers | ajudou | |
| Ticket `## Escopo` + bullets/`**` | parse OK | **ajudou** | contrastava com rodada 1 |
| `tb drift` / `tb review` | scope + completeness | ajudou / neutro | warning de aceite “pouco evidenciado” mesmo com testes no diff |
| `tb verify run` (build/lint/test/hidden) | PASS, 1 iter, ~0,84 s | **ajudou** | |
| `tb verify --ruler visual` | skipped=true agregado | ajudou | |
| `tb guard` + `guard.yaml` | violações custom | **ajudou** | |
| `tb guard --strict --json` | exit 1 | ajudou | |
| `tb bench run` sem `-t` | falha `bench/tasks` | atrapalhou | `tasks_dir` do YAML **não** aplicado; precisa `-t`/`-f` |
| `tb bench run -t …` sem `--agent` | usa default_agent | ajudou | |
| `tb bench compare --agent` | OK com `--config-a/b` | ajudou | |
| `tb mcp` tools/list + `tb_version` | OK | ajudou | stdio JSON-RPC; tools de grafo documentam `TB_GRAPH=on` |

## Problemas que continuam

| problema (rodada 1 / crush) | status | evidência |
|------------------------------|--------|-----------|
| `graph impact` exige `--file` | continua | exit 1 `--file é obrigatório` |
| Config `.toolbelt/*` mode **0600** | continua | `config.yaml` e `agent_docs.yaml` → 600 |
| `guard.yaml` manual **0644** vs configs tb 0600 | continua | `stat` → 644 |
| Docs/`mem add --type` na memória do agente | mitigado | CLI/docs OK; flag `--type` ainda unknown (esperado) |
| Verify tão bom quanto o `test_cmd` configurado | continua (por desenho) | precisei setar build/lint/test focados manualmente |
| Init não pré-preenche lint/build a partir do Taskfile/AGENTS | parcial | lê hints, mas **não** grava `lint_cmd`/`build_cmd` no YAML |
| Suite nativa completa / golangci não no VM | continua | não instalados |

Problemas da rodada 1 **resolvidos** neste run: SkipDirs raiz; guard.yaml ignorado; `--strict --json` exit 0; `default_agent` / `compare --agent`; Scope markdown; build/lint no verify; visual `skipped=false`; review FP `tests/hidden`; hidden gated só por dir; init cego a AGENTS.md.

## Problemas novos

### P2 — Extração de “hints” do AGENTS.md puxa blocos de código Go

- **Repro**: `tb init` na raiz do crush com `AGENTS.md` rico.
- **Expected**: dicas de comando (`go test`, `task lint:log`, …).
- **Actual**: `agent_docs.yaml` inclui funções inteiras (`CharmtonePantera`, template de teste mock) como `hints`, misturadas com comandos úteis.

### P2 — `bench.tasks_dir` / `fixtures_dir` do `config.yaml` não entram no `tb bench run`

- **Repro**: setar paths absolutos no YAML; rodar `tb bench run -r 1` sem `-t`/`-f`.
- **Expected**: usar dirs do config (como `default_agent`).
- **Actual**: `open bench/tasks: no such file or directory`; defaults relativos da CLI vencem.

### P3 — Review “completeness” avisa aceite já coberto por testes no mesmo commit

- **Repro**: `rd.md` com `- [x] go test ./internal/stringext/ passes` + diff com `string_test.go`; `tb review`.
- **Expected**: silêncio ou evidência positiva.
- **Actual**: warning `critério de aceite pouco evidenciado no diff`.

### P3 — Bench com `echo` como `default_agent` reportou 100% em `bugfix-go`

- **Repro**: `default_agent: echo AGENT-RAN {prompt}` + `tb bench run --task bugfix-go`.
- **Expected**: falha de verificação se o agente não edita o fixture.
- **Actual**: `passed: true` (output só ecoou o prompt). Possível gap do harness/fixture — não investigado a fundo neste repo.

## O que o tb pegou e o que deixou passar

**Pegou**

- Callers/impact de `Capitalize` via grafo Go (UI + agent).
- Drift quando o scope markdown era estreito demais (`string_test.go` fora).
- Guard custom em `string_test.go` e hardcoded em `tests/hidden` / `thresholds.yaml`.
- Verify com build + lint + test + hidden sem pasta `tests/hidden/`.
- Visual honestamente `skipped` no relatório raiz.
- Presença de `AGENTS.md` no init (path + tentativa de hints).

**Deixou passar / não cobriu**

- Não sugeriu automaticamente `task lint:log` / `go build` como `lint_cmd`/`build_cmd` a partir do AGENTS/Taskfile (só listou hints ruidosos).
- Não impediu (nem detectou) o uso de title-case em Capitalize *antes* da tarefa — o bug veio de inspeção humana + testes novos.
- Completeness warnings fracos (ruído, não bloqueio útil).
- `tasks_dir` do YAML ignorado no bench até passar `-t`.

Comparando com nativo: `go test ./internal/stringext/` + `task lint:log` + `go build` foram a fonte de verdade; o tb verify refletiu isso **depois** de eu configurar os cmds.

## Interação com instrumentação existente / onboarding

| Aspecto | Observação |
|---------|------------|
| `tb init` | Agora **detecta** `AGENTS.md` e grava `agent_docs.yaml`; toolchain Go correta. |
| `--write-agents` | Correto: não sobrescreve crush; semeia só quando ausente. |
| Qualidade do parse | Hints misturam code samples do guia com comandos reais — pouco acionável para preencher verify. |
| Skills `.agents/` | Continuam **fora** do onboarding do tb (não referenciadas). |
| Duplicação mem vs AGENTS | Invariantes genéricas do `mem init` ainda paralelas às regras de capitalização/CGO do AGENTS. |
| Graph | Indexa AGENTS aninhados (sections) e código Go com `calls` úteis. |

## Comparação com a rodada 1

| Dimensão | Rodada 1 (`c8aae04`) | Rodada 2 (`a46c295`) |
|----------|----------------------|----------------------|
| Tarefa | lint `slog.*Context` | `Capitalize` sentence-case |
| SkipDirs `/tmp/target` | quebra (0 nós se raiz=target) | **corrigido** (5183 nós) |
| Scope markdown | `scope-missing` | **parseia** `## Escopo` + bullets/`**` |
| guard.yaml | ignorado | **honrado** |
| `--strict --json` | exit 0 com violação | **exit 1** |
| build/lint no verify | mortos | **rodam** |
| hidden sem dir | ignorado | **executa** cmd |
| default_agent / compare --agent | quebrados | **ok** |
| AGENTS.md no init | ignorado | **lido** (hints ruidosos) |
| visual skipped | raiz mentirosa | **agregado** |
| Review FP tests/hidden | sim | **não** (só path real) |
| Utilidade geral | bootstrap frágil | **melhor** no harness; onboarding de hints ainda fraco |

Veredito: **melhor** de forma concreta nas 12 correções testáveis neste repo Go; novos atritos são menores (hints, `tasks_dir` do YAML, warnings de completeness).

## Sugestões de melhoria

1. **[P1][init]** Usar hints do AGENTS/Taskfile para **preencher** `verify.lint_cmd` / `build_cmd` (ex. `task lint:log`, `go build .`), não só listar texto.
2. **[P1][init]** Filtrar extração de hints: linhas/comandos curtos; ignorar fences/code blocks longos.
3. **[P1][bench]** Honrar `bench.tasks_dir` / `fixtures_dir` / `output_dir` do `config.yaml` como já faz `default_agent`.
4. **[P2][review]** Completeness: se o diff toca `*_test.go` alinhado ao aceite, não emitir warning genérico.
5. **[P2][graph]** Default de `impact` para arquivos do `git diff` quando `--file` omitido.
6. **[P3][config]** Documentar ou unificar permissões 0600 vs arquivos manuais em `.toolbelt/`.
7. **[P3][init]** Referenciar `.agents/skills/` no `agent_docs.yaml` quando existirem (grupo com instrumentação).

## Métricas

| Métrica | Rodada 2 | Rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (sessão instrumentada tb) | ~**2 min 30 s** (`12:39:55Z` → `12:42:24Z`), excl. build crush (~1 m16 s) e download toolchain | ~**3 min** |
| Invocações tb (`CMD:` no evidence) | **46** | ~30+ |
| Falhas tb (`EXIT=1`) | **14** (probes esperados: impact sem file, strict guard, drift estreito, hidden=false, bench sem `-t`, etc.) | **10** |
| Crashes tb | **0** | **0** |
| MCP | tools/list + `tb_version` OK | OK |
| Alterações no toolbelt | **somente** este arquivo `docs/campo/rodada2/crush.md` | só `docs/campo/crush.md` |
| Alterações no crush | locais em `/tmp/target/crush` (não push) | locais (não push) |
