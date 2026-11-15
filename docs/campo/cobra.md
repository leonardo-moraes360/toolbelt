# Estudo de campo: spf13/cobra

Grupo: **sem instrumentação de agente de IA**. Trabalho apenas em clone local (`/tmp/target/cobra`). Nenhuma interação com o upstream (sem push, issue, PR ou comentário).

`tb` usado: binário local de `/agent/repos/toolbelt` (`make build`, versão `c8aae04` com ldflags).

---

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | [spf13/cobra](https://github.com/spf13/cobra) |
| Linguagem | Go (`go 1.15` no `go.mod`; CI testa 1.17–1.24) |
| Tamanho | shallow clone ~1,3 MiB no disco; **36** arquivos `.go`; ~15–17 kLOC Go (wc nas fontes) |
| Instrumentação AI existente | **Nenhuma** (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot, Windsurf, Aider: ausentes no tree) |
| Toolchain | **Parcialmente ok**: `go test ./...` e `go vet` ok; `gofmt` ok; `golangci-lint` instalado via `go install` (script oficial falhou checksum); `make lint`/`richtest` dependem do linter; Docker/`addlicense` da CI **não** configurados; não rodei matriz multi-versão Go |

---

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Permitir `SetErrPrefix("")` (prefixo vazio, sem fallback para `"Error:"` e sem espaço leading na impressão) |
| Por quê | Feature pequena, API clara, testes naturais; maintainer discutiu no thread |
| Issue | https://github.com/spf13/cobra/issues/2226 |
| Completou? | **Sim, só localmente** (commit `a6ef947` no clone; **não** enviado ao GitHub) |
| Testes nativos | `go test ./...` → **PASS** (~0,2–1,1 s) |
| Lint nativo | `golangci-lint run ./...` → **FAIL** por issue **pré-existente** em `completions.go` (`nolintlint`/gosec); mudança ErrPrefix não introduziu achados |
| Build nativo | `go build ./...` → **PASS** no commit da tarefa |

Implementação: `errPrefix *string`, `printErrWithPrefix`, testes `TestEmptyErrPrefix` e `TestEmptyErrPrefixOnSubcommand`.

---

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | ok (0,006 s); cria `.toolbelt/config.yaml` mode **0600**, dirs `vault/`, `bench/`, `logs/` | **ajudou** (bootstrap mínimo) | `verify.test_cmd: go test ./...` útil; vault vazio; sem `AGENTS.md`/lint/build |
| `tb version` | com `make build`: `c8aae04`; `go build` sem ldflags: `0.1.0-dev`, **commit/built unknown** | neutro | confirma metadados só com ldflags |
| `tb mem init/list/add/check` | init gera 3 invariantes; add posicional ok; `--type/--title` **rejeitados** | **misto** | docs `cli.md` desatualizados vs CLI real (`tb mem add <regra>`) |
| ticket/`rd.md` + Scope | **não há** `tb ticket`; Scope só na **mesma linha** (`Scope: a, b`); lista Markdown captura só o 1º bullet (`- command.go`) | **atrapalhou** | `## Scope` + bullets → `scope-missing` ou scope parcial; `command_test.go`/`rd.md` marcados fora do escopo |
| `tb graph --enable index/stats/search/rebuild` | index 467 nós / 2108 arestas (~0,03–0,05 s); search achou `ErrPrefix`/`SetErrPrefix` | **ajudou** | localizou API rápido; sem `--enable`/`TB_GRAPH=on` falha (ADR-011) |
| `tb graph callers/deps/impact` | exigem `--function` / `--file` (args posicionais falham); impact útil mas ruidoso em `command.go` | **misto** | `impact` sem `--file` → exit 1; callers devolveu só o próprio nó |
| `tb verify run` (tests) | PASS no diff da tarefa (~0,2–0,5 s); só roda `test_cmd` (+ hidden); **ignora** `lint_cmd`/`build_cmd` no YAML | **misto** | com `go test ./standalone/...` → PASS enquanto `go build ./...` quebrava (`broken_syntax.go`) |
| `tb verify --ruler visual` | sem manifesto: PASS `skipped=true`; com manifesto e ref ausente: PASS **`skipped=false`** no relatório, screen `skipped=true` | **atrapalhou** | falso “não skipped” no agregador |
| `tb review` | fix-only limpo (findings `[]`); menção textual a `tests/hidden`/`thresholds.yaml` no diff → **FP critical** | **misto** | goldfish heurístico por substring, não por path tocado |
| `tb drift` | ok com `Scope: command.go, command_test.go, rd.md`; globs simples `command*.go` **funcionam** | **misto** | UX de Scope frágil; globs não são o pior problema |
| `tb guard check` | hardcoded `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`; **não lê** `.toolbelt/guard.yaml` | **misto** | proteção custom em `command.go` no YAML → **zero** violations |
| `tb verify` + path hardcoded | diff tocando `.toolbelt/thresholds.yaml` → verify **FAIL** imediato (iterações 0), sem rodar testes | **atrapalhou** (no fluxo de probe) | aborta mesmo com `guard.yaml` custom |
| `tb bench run/list/compare` | list ok; `--fake` ok **sem** `--fake-config`; `default_agent` no YAML **ignorado**; `compare` só tem `--fake`, **sem** `--agent` | **misto** | `Error: --agent ou --fake é obrigatório` com `default_agent` setado |
| `tb mcp` | stdio JSON-RPC ok; `tools/list` expõe graph/version/echo/env/run | **ajudou** (smoke) | stateless; graph tools exigem `TB_GRAPH=on` no processo servidor (não exercitado além do list) |
| checks nativos vs tb | tb verify PASS ≠ lint CI; narrow `test_cmd` esconde build break | **atrapalhou** se o agente confiar só no tb | ver seção “O que o tb pegou…” |

---

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --help` lista `--fake`, não `--agent` |
| `guard.yaml` ignored | **confirmado** | YAML protegia `command.go`; `guard check` no diff ErrPrefix → `violations: null` |
| outdated CLI docs | **confirmado** | `docs/cli.md` descreve `tb mem add --type/--title` e `tb ticket`; CLI: add posicional; `ticket` inexistente |
| version shows `unknown` | **confirmado** (sem ldflags) | `go build -o /tmp/tb-nolflags` → `commit: unknown`, `built: unknown` |
| config files created 0600 | **confirmado** | `.toolbelt/config.yaml` e `invariants.yaml` → `-rw-------` |
| visual ruler reports `skipped=false` when skipping | **confirmado** | manifesto presente, ref ausente: top-level `"skipped": false`, screen `"skipped": true` / `"referência ausente"` |
| `default_agent` ignored | **confirmado** | YAML `default_agent: echo AGENT_RAN {prompt}` → ainda exige `--agent` ou `--fake` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | toque em `.toolbelt/thresholds.yaml` → verify FAIL, `iterations: 0` |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake ... --task bugfix-go` PASS sem `--fake-config` |
| `mem add` positional | **confirmado** | help: `tb mem add <regra>`; `--type` → `unknown flag` |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `Scope: command*.go` aceitou os `.go`; falha real: listas Markdown multilinha / parser de uma linha |
| `graph impact` needs `--file` | **confirmado** | sem flag → `Error: --file é obrigatório` |
| review false positives on tests/hidden invariant | **confirmado** | comentário `// note about tests/hidden thresholds.yaml` → 2 findings critical sem alterar esses paths |
| verify runs only partial checks / misses build errors | **confirmado** | `test_cmd=go test ./standalone/...` PASS com `broken_syntax.go` no módulo; `go build ./...` FAIL |
| tb não empurra completar a tarefa toda | **confirmado (qualitativo)** | verify PASS com testes bastou; tb não cobrou lint/`golangci`, build dedicado, docs da API nem checklist de aceite; agente precisou lembrar checks nativos à parte |

---

## Problemas novos

### P1 — Parser de Scope ignora lista Markdown e engole o primeiro bullet via `\s`/newline

- **Repro**: `rd.md` com
  ```
  Scope:
  - command.go
  - command_test.go
  ```
  depois `tb drift --ticket rd.md --base <base> --head HEAD`
- **Esperado**: scope = ambos os arquivos (ou erro claro de formato)
- **Atual**: regex `(?mi)^(?:scope|escopo)\s*:\s*(.+)$` faz `\s*` cruzar newline; captura `- command.go`; `splitList` vira `["-", "command.go"]`; `command_test.go` sai como out_of_scope

### P2 — `tb init` não detecta lint/build do Makefile nem gera guia de agente

- **Repro**: `tb init` em cobra (tem `Makefile` com `test`/`lint`/`fmt`)
- **Esperado**: pelo menos sugerir `golangci-lint run` / `make test`, e/ou `AGENTS.md` mínimo
- **Atual**: só `verify.test_cmd: go test ./...`; vault vazio; sem lint/build; `lint_cmd`/`build_cmd` manuais no YAML são **ignorados** pelo `verify`

### P3 — `graph callers` / args posicionais inconsistentes com a UX de search

- **Repro**: `tb graph --enable callers ErrPrefix` vs `tb graph --enable search ErrPrefix`
- **Esperado**: mesma convenção posicional ou mensagem que cite a flag
- **Atual**: search aceita query posicional; callers/deps/impact exigem flags e falham de forma fácil de confundir com “grafo vazio”

### P3 — Campos extras em `config.yaml` (`lint_cmd`, `build_cmd`) silenciosamente inertes

- **Repro**: gravar `verify.lint_cmd` / `verify.build_cmd` e rodar `tb verify run`
- **Esperado**: usar, rejeitar schema, ou avisar
- **Atual**: verify só executa test/hidden; PASS sem lint

---

## O que o tb pegou e o que deixou passar

**Pegou**

- Localização da API ErrPrefix via `graph search`
- Drift quando Scope estava bem formatado na mesma linha
- Guard em paths hardcoded (`thresholds.yaml`, `tests/hidden/*`)
- Falha de `go test ./...` / build quebrado quando o `test_cmd` cobria o pacote
- MCP sobe e lista tools

**Deixou passar / não cobriu**

- Issue pré-existente do `golangci-lint` (verify não roda lint)
- Build quebrado se `test_cmd` for estreito (`./standalone/...`)
- Proteções declaradas em `guard.yaml` do usuário
- Completude “nível maintainer” (docs públicas da issue, changelog, multi-Go) — nenhum nudge
- Falso positivo de review por substring `tests/hidden` / `thresholds.yaml` no texto do diff

---

## Interação com instrumentação existente / onboarding

Repo **sem** instrumentação AI. Avaliação do bootstrap do zero:

| Artefato | Gerado? | Qualidade |
|----------|---------|-----------|
| `.toolbelt/config.yaml` | sim | defaults razoáveis para Go (`go test ./...`); graph languages/frameworks genéricos demais; `default_agent: ""`; hidden tests apontam para path inexistente até o agente criar |
| `.toolbelt/invariants.yaml` (via `mem init`) | sim | anti-cheat genérico (`tests/hidden`, `thresholds.yaml`) — úteis, mas geram FPs se o texto aparecer no diff |
| `.toolbelt/vault/` | dir vazio | sem notas de domínio Cobra |
| `AGENTS.md` / skills / Makefile-aware | **não** | falta o principal para repo sem instrumentação |
| Detecção automática test/lint/build | **parcial** | só chute `go test ./...`; não leu `Makefile`/`CI` |

Conclusão: onboarding **insuficiente** para um repo “nu”; o agente ainda precisa inventar ticket, Scope na sintaxe certa, e a suíte completa de qualidade.

---

## Sugestões de melhoria

1. **[P0][review/drift]** Aceitar Scope em lista Markdown / YAML frontmatter multi-item; rejeitar bullets virando token `"-"` — área: `internal/review`  
2. **[P0][verify]** Honrar `lint_cmd`/`build_cmd` (ou schema estrito + warning); default: detectar `Makefile`/workflow CI — área: `verify` + `init`  
3. **[P1][guard]** Carregar `.toolbelt/guard.yaml`; não abortar verify só por path hardcoded sem mandate — área: `guard`/`verify`  
4. **[P1][init/mem]** Gerar `AGENTS.md` + checklist (test/lint/build/Scope) e alinhar `docs/cli.md` ao CLI real — área: `init`, docs  
5. **[P1][verify/visual]** Relatório agregador: `skipped=true` se todas as screens skipparam — área: `internal/verify/visual.go`  
6. **[P2][review]** Matching de invariantes por path tocado, não substring no diff — área: `review.suggestsViolation`  
7. **[P2][bench]** Respeitar `bench.default_agent`; adicionar `--agent` em `bench compare` — área: `bench`  
8. **[P2][graph]** Unificar args posicionais (`callers`/`deps`/`impact`) com `search` — área: `graph` CLI  
9. **[P3][version/config]** Documentar ldflags no README; permissões 0644 para config não-secretos em clone compartilhado — área: build/`init`

---

## Métricas

| métrica | valor |
|---------|--------|
| Wall time (tb evidence log birth→last) | ~**3,6 min** (20:44:44 → 20:48:19 UTC); estudo completo incl. clone/build/lint install ~**6–8 min** |
| Soma tempos medidos das invocações tb | **~3,32 s** |
| Invocações tb registradas | **64** |
| Invocações com exit ≠ 0 | **19** (maioria probes de falha esperada / known issues; não crashes) |
| Crashes do tb | **0** observados |
| Commits locais no clone cobra (tarefa) | 1 funcional (`fix: allow empty ErrPrefix`) + commits de probe depois descartáveis |
| Interações com GitHub do alvo | **0** |
