# Estudo de campo (rodada 2): spf13/cobra

Grupo: **sem instrumentação de agente de IA**. Trabalho apenas em clone local (`/tmp/target/cobra`). Nenhuma interação com o upstream (sem push, issue, PR ou comentário).

`tb` usado: binário local de `/agent/repos/toolbelt` (`make build` a partir de `origin/main` @ `a46c295` com ldflags).

---

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | [spf13/cobra](https://github.com/spf13/cobra) |
| Linguagem | Go (`go 1.15` no `go.mod`) |
| Tamanho | shallow clone ~**1,3 MiB**; **36** arquivos `.go`; ~**16,8 kLOC** Go (`wc` nas fontes) |
| Instrumentação AI existente | **Nenhuma** no tree upstream (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot/Windsurf/Aider: ausentes). `tb init --write-agents` semeou `AGENTS.md` localmente. |
| Toolchain | **Parcialmente ok**: `go test ./...` PASS; `go build ./...` PASS; `go vet ./...` PASS; `gofmt` ok; `golangci-lint` **v2.1.6** instalado (v1 rejeitou `.golangci.yml` v2) → **1 issue pré-existente** `nolintlint` em `completions.go`; Docker/`addlicense` da CI **não** configurados; sem matriz multi-Go |

---

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Incluir `CommandPath()` nas mensagens de erro de `ExactArgs` / `MinimumNArgs` / `MaximumNArgs` / `RangeArgs` (ex.: `command "root child" accepts 1 arg(s), received 2`) |
| Por quê | Feature pequena, alinhada ao estilo de `NoArgs`/`OnlyValidArgs` (já citam o path); testes naturais; **diferente** da rodada 1 |
| Issue | Nenhuma (escolha local; sem abrir issue no upstream) |
| Completou? | **Sim, só localmente** (commit `e2643be` no clone; **não** enviado ao GitHub) |
| Testes/lint/build nativos | `go test ./...` → **PASS** (~0,2–1,1 s); `go build ./...` → **PASS**; `gofmt -l` limpo; `golangci-lint run ./...` → **FAIL** pré-existente (`completions.go`), não introduzido pela mudança |
| Rodada 1 (comparação) | Rodada 1: `SetErrPrefix("")` (issue [#2226](https://github.com/spf13/cobra/issues/2226)). Esta rodada: path do comando nos erros de contagem de args posicionais |

Implementação: `args.go` + asserts em `args_test.go` + `TestExactArgsErrorIncludesCommandPath`.

---

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|-----------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph --enable index /tmp/target` → **463 nós / 2103 arestas** (antes: 0 nós quando a raiz se chamava `target`) |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | sem dir + `hidden_test_cmd: "echo HIDDEN_RAN && false"` → `hidden_available: true`, verdict **ESCALATE**; com `echo HIDDEN_OK` → **PASS** / `hidden_passed: true` |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | `tb guard check BASE..HEAD --strict --json` com violação → stdout JSON + **EXIT:1**; sem `--strict` + `--json` → EXIT:0 (esperado) |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | YAML com `args.go` → `protected_path` em `args.go`; trocando para só `command.go` (intocado) → `violations: null` |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em cobra → `verify.test_cmd: go test ./...`; `hidden_test_cmd: ""` (não mais default Go hidden cego) |
| 6 | review `tests/hidden` por path | **funcionou** | ticket `rd.md` menciona `tests/hidden` / `thresholds.yaml` no texto; review no diff da tarefa → **sem** finding critical de invariante; só drift/`completeness` |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | YAML `default_agent: "echo AGENT_RAN {prompt}"` + `tb bench run -r 1` **sem** `--agent`/`--fake` → sucesso 100%; `tb bench compare --help` expõe `-a/--agent` |
| 8 | review/drift `## Scope`/`## Escopo`, listas MD e `**` | **funcionou** | `## Escopo` + bullets + `**/*args*.go` no `rd.md` → drift só marcou `rd.md` fora; tickets `/tmp/rd-scope-test.md` e glob `**/*args*.go` → `out_of_scope: null` no commit da feature |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | config com `build_cmd: go build ./...` e `lint_cmd: gofmt -l .` → JSON `build_ran/passed: true`, `lint_ran/passed: true` |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** | sem arquivos: `--write-agents` semeou `AGENTS.md` + `agent_docs.yaml`; probe com AGENTS+CLAUDE existentes → `sources: [AGENTS.md, CLAUDE.md]` e **não** sobrescreveu |
| 11 | graph Python `calls` | **funcionou** (probe fora do cobra) | fixture `/tmp/py-calls-probe/a.py` (`main`→`helper`): `edges_calls: 1`; `tb graph callers --function helper` → `func:a.main`. Em cobra (Go) já havia `edges_calls: 1470` via AST Go |
| 12 | verify visual skipped agregado + docs CLI alinhados | **funcionou** | manifesto com ref ausente → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"`; `docs/cli.md` documenta `mem add <regra>`, `## Scope`, `protected_paths`, `default_agent`, `hidden_test_cmd` |

---

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` / `--write-agents` | ok (~0,004 s); config + `AGENTS.md` + `agent_docs.yaml` | **ajudou** | toolchain Go detectada; seed útil num repo sem instrumentação |
| `tb version` | `a46c295` com ldflags | neutro | metadados corretos no binário do estudo |
| `tb mem init/list/add/check` | init 3 invariantes; add posicional ok; check retenção 1.00 | **ajudou** | alinhado ao `docs/cli.md` atual (`tb mem add <regra>`) |
| ticket `rd.md` + `## Escopo` | parser aceita heading + bullets + glob `**` | **ajudou** | drift/review enxergaram `args.go`/`args_test.go` no escopo |
| `tb graph --enable index/stats/search/rebuild/impact` | index 463/2103; search achou `ExactArgs`; impact em `args.go` listou callers | **ajudou** | SkipDirs na raiz corrigido; search localizou a API |
| `tb graph callers` | exige `--function` (flag `--file` **removida**/ausente) | **misto** | UX ainda assimétrica vs `search` posicional; funciona com `--function ExactArgs` falhou sem índice de calls no nome curto — com Python probe ok |
| `tb verify run` (+ build/lint/hidden) | PASS com build/lint; hidden respeita cmd sem dir | **ajudou** | falso PASS da rodada 1 (só test_cmd) mitigado quando lint/build configurados |
| `tb verify --ruler visual` | PASS `skipped=true` (manifesto ausente ou refs ausentes) | neutro | agregação honesta; pouco valor num CLI Go sem UI |
| `tb review` | fix-only: warning `completeness` fraco; com `rd.md` no diff: drift error | **misto** | sem FP de `tests/hidden`; ainda sinaliza aceite “go test passes” sem evidência forte no diff |
| `tb drift` | ok com `## Escopo`; `rd.md` fora do escopo → exit 1 | **ajudou** | globs `**` ok; ticket fora do scope é ruído se o agente commitá-lo |
| `tb guard check BASE..HEAD` | lê `guard.yaml`; `--strict --json` → exit 1 | **ajudou** | sintaxe posicional `base..head` (não `--base`); custom paths valem |
| `tb bench run/list/compare` | default_agent honrado; compare tem `--agent`; `--fake` ok | **ajudou** | campo morto da rodada 1 corrigido |
| `tb mcp` | stdio JSON-RPC; `tools/list` com graph/version/echo/env/run | **ajudou** (smoke) | com `TB_GRAPH=on` lista tools de grafo |
| checks nativos vs tb | tb verify PASS ≠ `golangci-lint` (issue pré-existente) | **misto** | init **não** preenche `lint_cmd` com `golangci-lint`/`make lint` automaticamente |

---

## Problemas que continuam

| problema (rodada 1 / síntese) | status nesta rodada | evidência |
|-------------------------------|---------------------|-----------|
| Config/arquivos gerados em mode `0600` | **continua** | `.toolbelt/config.yaml` e `AGENTS.md` → `-rw-------` |
| `version` unknown sem ldflags | **não retestado** (binário do estudo usou `make build`) | esperado pelo design de build |
| Guard/verify abortam ao tocar `.toolbelt/thresholds.yaml` | **continua** (anti-cheat intencional) | probe: verify **FAIL** `iterations: 0` ao commitar `thresholds.yaml` |
| Init não preenche lint/build a partir do Makefile/CI | **continua** | só `test_cmd: go test ./...`; `lint_cmd`/`build_cmd` exigem edição manual |
| `graph callers` UX inconsistente com `search` | **continua** (parcial) | `callers` ainda flag-only (`--function`); `--file` ausente no help atual |
| tb não empurra lint nativo “nível maintainer” | **continua** | verify PASS com `gofmt` enquanto `golangci-lint` falha no repo |
| Extração de hints de `AGENTS.md`/`CLAUDE.md` | **continua fraca** | `agent_docs.yaml` → `hints: []` mesmo com texto “Use make test” / “Prefer pytest” |

---

## Problemas novos

### P2 — `tb init` não sugere `golangci-lint`/`make lint` mesmo com Makefile + `.golangci.yml`

- **Repro**: `tb init` em `/tmp/target/cobra` (tem `Makefile` target `lint` e `.golangci.yml`)
- **Esperado**: preencher ou sugerir `lint_cmd` (e talvez `build_cmd`)
- **Atual**: só `verify.test_cmd: go test ./...`; lint/build ficam a cargo do agente

### P3 — Review `completeness` alerta “go test ./... passes” sem olhar o resultado do verify

- **Repro**: `tb review --ticket rd.md --base <base> --head e2643be` após testes verdes
- **Esperado**: critério de teste satisfeito se o harness/verify passou, ou ausência de warning
- **Atual**: finding `warning` / `accept-2` “critério de aceite pouco evidenciado no diff”

### P3 — Manifesto visual ignora `reference:` custom e usa `references/<screen>-light.png`

- **Repro**: `.toolbelt/screens.yaml` com `reference: .toolbelt/refs/missing.png` + `tb verify run --ruler visual --manifest ...`
- **Esperado**: usar o path declarado (ou erro claro citando-o)
- **Atual**: `skip_reason: referência ausente` apontando `references/home-light.png`

### P3 — Commitar `rd.md` fora do `## Escopo` vira drift/review error mesmo com a feature in-scope

- **Repro**: escopo só `args*.go` + commit do ticket no mesmo `HEAD`
- **Esperado**: documentação de ticket ignorada ou padrão para incluí-la no scope
- **Atual**: `out_of_scope: ["rd.md"]` e review exit ≠ 0

Nenhuma regressão P0/P1 observada nas 12 correções testadas.

---

## O que o tb pegou e o que deixou passar

**Pegou**

- Localização de `ExactArgs` via `graph search`
- Drift quando o scope Markdown estava bem formado (incluindo glob `**`)
- Guard em `protected_paths` custom (`args.go`) e defaults (`thresholds.yaml`)
- Falha de hidden suite configurada sem pasta `tests/hidden/`
- Build/lint configurados no verify (quando o agente preenche o YAML)
- FP de review por substring `tests/hidden` **não** voltou
- MCP sobe e lista tools

**Deixou passar / não cobriu**

- Issue pré-existente do `golangci-lint` (verify com `gofmt` ≠ lint CI)
- Completude “nível maintainer” (changelog upstream, docs públicas, multi-Go) — sem nudge
- Hints acionáveis a partir de `AGENTS.md`/`CLAUDE.md` (`hints: []`)
- Qualidade real de UI (ruler visual always-skip neste repo)

---

## Interação com instrumentação existente / onboarding

Repo **sem** instrumentação AI no upstream. Bootstrap do zero (pós-fixes):

| Artefato | Gerado/lido? | Qualidade |
|----------|--------------|-----------|
| `.toolbelt/config.yaml` | gerado | `test_cmd` Go correto; `hidden_test_cmd` vazio (melhor que default cego); lint/build **não** detectados |
| `.toolbelt/agent_docs.yaml` | gerado | registra sources; `hints: []` |
| `AGENTS.md` via `--write-agents` | gerado | esqueleto mínimo apontando `tb verify`/`guard`/`mem` — útil, genérico |
| `.toolbelt/invariants.yaml` (`mem init`) | gerado | anti-cheat padrão; agora casa por path no review |
| `.toolbelt/guard.yaml` | manual | **honrado** quando criado |
| Leitura de AGENTS/CLAUDE pré-existentes | ok no probe | não sobrescreve com `--write-agents` |

Conclusão: onboarding **melhorou** vs rodada 1 (seed de AGENTS + toolchain + guard/scope/verify), mas ainda exige o agente configurar lint/build e o ticket no formato certo.

---

## Comparação com a rodada 1

| área | rodada 1 | rodada 2 | veredito |
|------|----------|----------|----------|
| Grafo em `/tmp/target` | risco de 0 nós (SkipDirs) | 463 nós na raiz `target` | **melhor** |
| Scope Markdown | bullets quebravam drift | `## Escopo` + listas + `**` ok | **melhor** |
| `guard.yaml` | ignorado | `protected_paths` aplicado | **melhor** |
| `--strict --json` | exit 0 com violação | exit 1 | **melhor** |
| `hidden_test_cmd` | gated por dir | roda sem dir | **melhor** |
| `build_cmd`/`lint_cmd` | inertes | executados | **melhor** |
| `default_agent` / `compare --agent` | morto / ausente | honrados | **melhor** |
| Review FP `tests/hidden` | confirmado | não reproduziu | **melhor** |
| Init AGENTS | não lia/gerava | `--write-agents` + leitura | **melhor** |
| Visual skipped | agregado mentia | `skipped: true` agregado | **melhor** |
| Docs CLI | desatualizados | exemplos batem com CLI testada | **melhor** |
| Detecção lint/build automática | ausente | ainda ausente | **igual** |
| Completude maintainer / golangci | lacuna | lacuna | **igual** |
| Config mode 0600 | confirmado | confirmado | **igual** |

No geral: **claramente melhor** nas 12 correções; lacunas de onboarding profundo (Makefile→lint) e de juízo de completude permanecem.

---

## Sugestões de melhoria

1. **[P1][init/verify]** Detectar `Makefile`/`golangci`/workflows e preencher ou sugerir `lint_cmd`/`build_cmd` — área: `internal/config/detect.go`
2. **[P2][init/mem]** Extrair `hints` reais de `AGENTS.md`/`CLAUDE.md` (comandos de test/lint) — área: init/`agent_docs`
3. **[P2][review]** Amarrar critérios de aceite de teste ao resultado de `tb verify` / presença de `*_test.go` no scope — área: `internal/review`
4. **[P3][verify/visual]** Honrar campo `reference` do manifesto (ou documentar o path canônico `references/`) — área: `internal/verify/visual.go`
5. **[P3][drift/review]** Tratar `rd.md`/ticket path como in-scope por default ou documentar inclusão no `## Scope` — área: drift/review
6. **[P3][graph]** Aceitar query posicional em `callers` como em `search`, ou mensagem que cite `--function` — área: `cmd/tb/graph.go`
7. **[P3][config]** Mode `0644` para configs não-secretos gerados (`config.yaml`, `AGENTS.md`) em clones compartilhados — área: init

---

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (log de evidências tb) | ~**2,8 min** (12:37:00 → 12:39:48 UTC); estudo completo com install golangci-lint v2 ~**5–7 min** | ~3,6 min (tb) / ~6–8 min total |
| Invocações `tb` registradas | **40** | **64** |
| Invocações `tb` com exit ≠ 0 | **9** (probes esperados: guard strict, drift/review fora de scope, hidden fail, flags erradas) | **19** |
| Crashes do tb | **0** | **0** |
| Commits locais no clone cobra | 1 feature (`e2643be`) + 1 ticket `rd.md` (`3f01345`); probes de thresholds descartados | 1 funcional + probes |
| Interações com GitHub do alvo | **0** | **0** |
| Binário tb | `a46c295` | `c8aae04` |
