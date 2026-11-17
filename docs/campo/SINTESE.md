🇧🇷 Português | [🇺🇸 English](../en/campo/SINTESE.md)

# Síntese do estudo de campo do `tb` (20 repositórios)

> Data: 2026-11-15 · Branch: `campo/sintese` · Binário típico dos agentes: `toolbelt@c8aae04`  
> Relatórios brutos: [`docs/campo/<slug>.md`](.) · Pesquisa: [`docs/pesquisa/`](../pesquisa/) · ADRs: [`docs/adr/`](../adr/)

Este documento agrega os 20 relatórios de campo, verifica as claims mais fortes contra o código atual do `tb`, cruza com a pesquisa em `docs/pesquisa/`, e elege melhorias ranqueadas para PRs focados (sem implementar código nesta entrega).

---

## (a) Método e tabela dos 20 repositórios

### Método

1. **Amostra**: 20 clones locais de repositórios públicos (10 com instrumentação de agente de IA já existente; 10 sem).
2. **Protocolo**: cada cloud agent usou o `tb` no clone, escolheu uma tarefa pequena/média maintainer-plausível, rodou a bateria de comandos (`init`, `mem`, `graph`, `drift`, `review`, `verify`, `guard`, `bench`), e publicou **um** relatório em `docs/campo/<slug>.md` na branch `campo/<slug>`.
3. **Síntese**: esta branch traz os 20 arquivos **inalterados**, relê pesquisa/ADRs/README, e **revalida no código** as claims recorrentes (lista na seção de verificação).
4. **Limitações**: tarefas e tooling nativo variam; alguns checks nativos foram parciais por custo; o clone em `/tmp/target` enviesou a descoberta do bug `SkipDirs`; interação com upstream foi proibida.

### Grupos

| Grupo | Slugs |
|-------|-------|
| **Com** instrumentação AI (`AGENTS.md` / `CLAUDE.md` / skills / etc.) | playwright-mcp, ollama, k6, excalidraw, ky, mcp-python-sdk, crush, sentry-python, ruff, pydantic-ai |
| **Sem** instrumentação AI | cobra, gin, ripgrep, bat, typer, click, requests, express, commander-js, jq |

### Tabela resumo

| Slug | Linguagem | Grupo | Tarefa (resumo) | Completa? | Checks nativos (resultado) |
|------|-----------|-------|-----------------|-----------|----------------------------|
| [playwright-mcp](playwright-mcp.md) | JS/TS | com | Docs de env vars em `update-readme.js` + regressão CLI | Sim (local) | `playwright test` 2 passed; `npm run lint` OK |
| [ollama](ollama.md) | Go (+TS/C++) | com | Flag `verbose` ausente em `ollama launch` | Sim (local) | `go test ./cmd/` OK; build cmake nativo falhou no VM |
| [k6](k6.md) | Go | com | `TagSet.Len` / `Iterate` | Parcial | `go test ./metrics/` + build + vet OK; lint/`-race` não rodados |
| [excalidraw](excalidraw.md) | TS/React | com | `normalizeRadians` → `0` em múltiplos de 2π | Sim (local) | vitest math PASS; typecheck PASS |
| [ky](ky.md) | TS | com | Rejeitar `retry.jitter` inválido | Sim (local) | `npm run build` OK; testes de retry OK |
| [mcp-python-sdk](mcp-python-sdk.md) | Python | com | Default `encoding_error_handler=replace` | Sim (local) | pytest stdio 37+2 passed; ruff OK |
| [crush](crush.md) | Go | com | Lint de capitalização `slog.*Context` | Sim (local) | scripts + `task lint:log` + `go build` OK |
| [sentry-python](sentry-python.md) | Python | com | `EventScrubber` hífen/underscore + denylist | Sim (local) | tox scrubber 15 passed; ruff OK |
| [ruff](ruff.md) | Rust | com | FURB101/103 não “fix” `open(b"...")` | Sim (local) | `cargo test -p ruff_linter …refurb` PASS |
| [pydantic-ai](pydantic-ai.md) | Python | com | `MessagesBuilder.last_modified` | Sim (local) | ruff/pyright OK; pytest alvo PASS |
| [cobra](cobra.md) | Go | sem | `SetErrPrefix("")` | Sim (local) | `go test ./...` PASS; lint com issues pré-existentes |
| [gin](gin.md) | Go | sem | Nome do campo em erros de BindUnmarshaler | Sim (local) | `go test`/`build` PASS; lint pré-existente no pacote |
| [ripgrep](ripgrep.md) | Rust | sem | Linhas erradas com `-U`+`-r` multilinha | Sim (local) | testes printer + suite alvo PASS; fmt OK |
| [bat](bat.md) | Rust | sem | Validar `--language`/`--map-syntax` com color never | Sim (local) | build/fmt/clippy + testes integração alvo OK |
| [typer](typer.md) | Python | sem | Flaky `test_file_error` com `TMPDIR` longo | Sim (local) | pytest arquivo PASS; ruff OK |
| [click](click.md) | Python | sem | `get_error_hint` + `auto_envvar_prefix` | Sim (local) | 2627 passed; ruff/mypy nos tocados OK |
| [requests](requests.md) | Python | sem | Digest `qop` com espaços após vírgula | Sim (local) | pytest digest 4 passed; ruff OK |
| [express](express.md) | JS | sem | `req.subdomains` com IPv6 mapeado | Sim (local) | `npm test` + lint PASS |
| [commander-js](commander-js.md) | JS/TS | sem | `Help.groupItems` Map→record + alias tipado | Sim (local) | `npm test` + `npm run check` PASS |
| [jq](jq.md) | C | sem | Rejeitar literais `23.` / `-23.` / `0.` | Sim (local; 1ª tentativa falhou) | `make check` 9/9 PASS |

**Taxa de conclusão local**: 19/20 sim; 1/20 parcial (k6). Nenhuma tarefa foi enviada ao upstream (protocolo do estudo).

---

## Verificação das claims no código atual

Cada claim abaixo foi confrontada com o source em `main`/`campo/sintese` (pacotes citados).

| # | Claim | Veredito | Evidência no código |
|---|-------|----------|---------------------|
| 1 | `SkipDirs` contém `target` e aplica-se ao **basename da raiz** do walk → indexar `/tmp/target` rende 0 nós | **Verificado** | `internal/graph/indexer.go`: `SkipDirs` inclui `"target"`; `walkSourceFile` faz `if skipDirs[info.Name()] { return filepath.SkipDir }` sem excluir a raiz |
| 2 | `tb init` grava `verify.test_cmd: go test ./...` por default | **Verificado** | `internal/config/config.go` `DefaultConfig().Verify` |
| 3 | `.toolbelt/guard.yaml` é ignorado | **Verificado** | `internal/guard/guard.go` `NewChecker` só hardcoda `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml`; nenhum loader de YAML |
| 4 | `bench.default_agent` é ignorado | **Verificado** | campo existe em `BenchConfig`, mas `parseBenchRunFlags` em `cmd/tb/bench.go` exige `--agent`/`--fake` e não lê o YAML |
| 5 | `tb verify` não roda build/lint (só `test_cmd` + hidden) | **Verificado** | `internal/verify/verify.go` `Run` chama apenas `VisibleTest` / `HiddenTest` |
| 6 | `hidden_test_cmd` só executa se existir `tests/hidden/` | **Verificado** | `verify.go`: `HiddenAvailable` via `os.Stat(.../tests/hidden)`; hidden só roda se true |
| 7 | `tb review` FP na substring literal `tests/hidden` | **Verificado** | `suggestsViolation` em `internal/review/review.go` faz `strings.Contains` em rule+diff |
| 8 | `tb guard check --strict --json` pode sair 0 com violação | **Verificado** | `cmd/tb/guard.go`: com `jsonOutput` retorna **antes** do bloco `strict`/`os.Exit(1)` |
| 9 | Ferramentas de grafo no MCP/CLI exigem `TB_GRAPH=on` / `--enable` | **Verificado** | `cmd/tb/graph.go`, `internal/graph/graph.go` `EnvEnable = "TB_GRAPH"`; ADR-011 |
| 10 | Python/Rust quase sem arestas `calls` | **Verificado** | `regexLang.IndexFile` em `index_common.go` só emite `defines`/`imports`; `EdgeCalls` só em `index_go.go` / `index_javascript.go` |
| 11 | `init`/`mem` não leem nem geram `AGENTS.md`/`CLAUDE.md` | **Verificado** | nenhum match em código Go; `EnsureDefaultAntiCheat` só grava 3 invariantes fixas |
| 12 | Parser Markdown de ticket: `## Scope` + bullets frágil; globs `**` parciais | **Verificado** | `parseTicketMarkdown` só casa `scope:`/`escopo:` na mesma linha; `inScope` trata sufixo `/**` e `filepath.Match`, não heading Markdown nem `src/**/*.rs` de forma geral |
| 13 | Arquivos de config criados como `0600` | **Verificado** | `config.Write` / saves usam `0o600` |
| 14 | Ruler visual: `skipped=false` no topo com telas skipped | **Parcial** | `skipVisual` (manifesto/browser ausente) seta `Skipped=true`; se o manifesto existe e só faltam referências, telas ficam `skipped=true` mas o relatório **não** propaga `report.Skipped=true` |

Claims dos checklists de campo que o código **não** sustenta como bug:

| Claim | Veredito | Nota |
|-------|----------|------|
| Drift/Scope só aceitam paths literais (sem globs) | **Refutado no código** (e na maioria dos relatórios) | `inScope` implementa globs simples |
| `--fake` exige `--fake-config` | **Refutado** | caminho feliz funciona sem a flag |
| Grafo sempre on / sem flag | **Refutado por desenho** | ADR-011: sob demanda |

---

## (b) Matriz de frequência (issues × repositórios)

Contagens a partir das tabelas “problemas conhecidos” dos 20 relatórios, normalizadas. Formato: **confirmado / refutado / parcial / não testado**.

### Issues do checklist compartilhado

| Issue | Com AI (n=10) | Sem AI (n=10) | Total confirmado | Código |
|-------|---------------|---------------|------------------|--------|
| `bench compare` sem `--agent` | 10/0/0/0 | 10/0/0/0 | 20 | confirmado (CLI) |
| `guard.yaml` ignorado | 10/0/0/0 | 10/0/0/0 | 20 | **verificado** |
| Docs CLI desatualizados | 10/0/0/0 | 10/0/0/0 | 20 | docs vs cobra |
| `version` → `unknown` sem ldflags | 10/0/0/0 | 10/0/0/0 | 20 | esperado sem `-ldflags` |
| Config files mode `0600` | 10/0/0/0 | 10/0/0/0 | 20 | **verificado** |
| Visual `skipped=false` ao pular | 8/2/0/0 | 5/5/0/0 | 13 | **parcial** (cenário referências) |
| `default_agent` ignorado | 10/0/0/0 | 10/0/0/0 | 20 | **verificado** |
| Guard aborta verify em path hardcoded | 10/0/0/0 | 10/0/0/0 | 20 | **verificado** (anti-cheat) |
| Fake precisa `--fake-config` | 2/8/0/0 | 1/8/1/0 | 3 | **refutado** na prática |
| `mem add` só posicional | 10/0/0/0 | 10/0/0/0 | 20 | docs desatualizados |
| Drift exige paths literais | 5/5/0/0 | 0/10/0/0 | 5 | **refutado** no código; confusão com `## Scope` |
| `graph impact` exige `--file` | 10/0/0/0 | 10/0/0/0 | 20 | UX, não bug |
| Review FP em `tests/hidden` | 8/1/0/1 | 8/2/0/0 | 16 | **verificado** |
| Verify checks parciais / miss build | 10/0/0/0 | 10/0/0/0 | 20 | **verificado** |
| tb não empurra completar a tarefa | 9/0/0/1* | 8/0/0/2 | ~17 | qualitativo; alinhado a lacuna de harness |

\*valores arredondados a partir das tabelas; alguns relatórios marcaram N/A.

### Issues novas (emergiram no campo)

| Issue | Com AI | Sem AI | Notas de campo | Código |
|-------|--------|--------|----------------|--------|
| `SkipDirs` + raiz `target` → 0 nós | ~4 confirmados (onde o clone era `/tmp/target`) | ~5 | Reproduzível; indexar subdir funciona | **verificado** |
| `hidden_test_cmd` gated por dir | ~9 | ~9 | Quase universal | **verificado** |
| Init default Go em repo não-Go | ~9 | 10 | Mais doloroso no grupo **sem** AI | **verificado** |
| Parser `## Scope` / bullets | ~2–4 | ~1–3 | Forte em Go (cobra/crush) | **verificado** |
| `guard --strict --json` exit 0 | poucos exercitaram | poucos | Bug claro quando testado | **verificado** |
| Poucas arestas `calls` (Py/Rust) | Py/Rust do grupo com | Py/Rust/C do grupo sem | callers/impact vazios | **verificado** |
| Não lê/gera `AGENTS.md` | 10 (há arquivo e tb ignora) | 10 (não gera) | Grupo **com** perde skills humanas | **verificado** |
| Grafo off sem `TB_GRAPH`/`--enable` | comum | comum | Alinhado ADR-011 | **verificado** (desejado) |

Diferença **com vs sem** instrumentação: os bugs de produto são quase idênticos. O grupo **sem** sofre mais no onboarding (`go test` default + ausência de `AGENTS.md`). O grupo **com** mostra que o tb **não aproveita** a instrumentação já presente (skills, comandos de lint documentados).

---

## (c) O que o tb pegou vs o que deixou passar

### Pegou (ajudou de verdade)

- **Guard anti-cheat em paths hardcoded**: toque em `tests/hidden/` ou `.toolbelt/thresholds.yaml` → verify FAIL cedo (vários repos).
- **Drift / review de escopo** quando o ticket usa `scope:` na mesma linha ou frontmatter YAML: flagrou `rd.md` e arquivos fora do mandate.
- **Mem invariantes**: baseline anti-cheat + `mem check` de retenção.
- **Graph search** em Go/JS quando o índice não era vazio: localizou símbolos (cobra `SetErrPrefix`, express `subdomains`, etc.).
- **Verify** como runner do `test_cmd` configurado manualmente (mocha/pytest/go test/cargo subset).

### Deixou passar / atrapalhou

- **PASS de verify sem lint/build/typecheck** (commander, express, bat com `test_cmd=true`, etc.).
- **Hidden suite configurada mas nunca executada** sem pasta `tests/hidden/`.
- **Grafo mudo** em clones `/tmp/target` e **callers/impact vazios** em Python/Rust/C/shell.
- **FP de review** quando o próprio ticket menciona `tests/hidden`.
- **guard.yaml custom inútil**; exit code frouxo com `--json`.
- **Init Go-cêntrico** em 18+ repos não-Go ou mistos; não lê `AGENTS.md`/`CLAUDE.md`/`package.json`/`Makefile`.
- **Não empurra completude** (k6 ficou parcial; jq precisou de 2ª tentativa sem nudge de harness).

### Padrão agregado

O `tb` hoje é um **bom conjunto de réguas pontuais** (guard hardcoded, drift literal, verify de um comando) e um **mau bootstrapper / mau aproveitador de contexto humano**. Nos repos com instrumentação, a lacuna é de *integração*; nos sem, de *descoberta*.

---

## (d) Comparação com `docs/pesquisa`

### Achados de campo **apoiados** por fontes/ideias da pesquisa

| Achado de campo | Fonte / ideia | Ligação |
|-----------------|---------------|---------|
| Suíte oculta frágil / fácil de contornar (`hidden` gated) | SpecBench, ImpossibleBench, achado #2 | Held-out só vale se de fato roda e fica invisível ao gerador |
| Review/guard com FP e padrões frágeis | TRACE, ImpossibleBench, ADR-005/008 | Precisa de sinais melhores que substring |
| Init sem progress/AGENTS/features | Anthropic *Effective harnesses*; `AGENTS.md` spec; achados tabela | Bootstrap incompleto = agente sem trilho |
| Grafo às vezes inútil/caro; off por default OK | *How Should Agents See Code?*, Cost-Inefficient Behaviors, ADR-011 | Campo confirma: search ajuda; callers/impact fracos; forçar grafo não |
| Skills humanas > busca automática | Cost-Inefficient Behaviors (−41% tokens) | Repos **com** AI tinham skills/`AGENTS.md` que o tb ignorou |
| Invariantes fora da compactação já existem (`tb mem`) | Compact-Bench, arXiv 2610.31778, ADR-010 | Campo usou `mem`; falta amarrar a review sem FP |
| Escopo declarado no ticket | Claim Plane, CAID, achado #5 | Drift ajudou quando o parser entendeu o scope |
| Juízes precisam sinal externo | Huang et al.; MT-Bench | Verify sem lint = falso PASS; juízes LLM sozinhos não bastam |

### Ideias de pesquisa que o campo mostra como **urgentes**

1. **Harness de longo prazo / `tb init` estilo progress+features** — onboarding falhou em quase todos os repos sem (e também com) instrumentação.
2. **Held-out real no verify** — o gating por `tests/hidden/` anula o KPI de reward hacking.
3. **Consumir `AGENTS.md` / skills humanas** — evidência de campo + paper de custo.
4. **Monitor além do diff** — guard só olha paths/AST Go; padrões `__eq__` etc. quase não apareceram no scanner multi-linguagem.
5. **Parser de mandate/scope robusto** — sem isso, Claim Plane no papel não vira enforcement.

### Ideias de pesquisa **sem suporte** (ou sem evidência) neste campo

| Ideia | Por quê o campo não sustenta (ainda) |
|-------|--------------------------------------|
| Router / A2A / workers paralelos / watchdog de desperdício | Nenhuma tarefa usou multi-agente; não há evidência de colisão de escopos neste estudo |
| ACP (Zed) / UI TUI `tb md` | Fora do protocolo dos 20 agentes |
| Métricas visuais Design2Code / CLIP | Ruler visual quase sempre skipped; não gerou sinal útil |
| Índice vetorial / Graphify integração profunda | Grafo regex já é o gargalo; vetor não foi a falha observada |
| Sandbox proxy de rede / Landlock | Pouco exercitado nas tarefas de bugfix unitário |
| `tb bench --impossible` | Bench só rodou `--fake` no toolbelt; não mediram taxa de trapaça no alvo |
| Mutação de testes (ACH) | Nenhum repo usou mutantes |
| Compactação híbrida em produção | `mem check` existe, mas não houve sessões longas com compactação LLM |

---

## (e) Melhorias eleitas (P0→P3) e PRs propostos

### Ranking

#### P0-1 — Não pular a raiz do walk quando o basename ∈ `SkipDirs`

- **Problema**: `target`/`build`/`dist` como nome da pasta raiz zera o grafo sem erro.
- **Evidência**: confirmado em clones `/tmp/target` — [click](click.md), [commander-js](commander-js.md), [express](express.md), [ky](ky.md), [mcp-python-sdk](mcp-python-sdk.md), [requests](requests.md), [sentry-python](sentry-python.md), [typer](typer.md), [k6](k6.md) (≈9).
- **Fonte**: ADR-011 (grafo útil sob demanda); papers de mapa de código.
- **Mudança**: em `walkSourceFile`, não aplicar `SkipDir` quando `path == root`.
- **Pacote**: `internal/graph` (+ teste).
- **Aceite**: `BuildFromRoot("/tmp/target")` indexa filhos; `node_modules`/`target` **filhos** continuam pulados; teste de regressão.
- **Tamanho**: S

#### P0-2 — Executar `hidden_test_cmd` quando configurado (sem exigir dir)

- **Problema**: comando hidden no YAML/`--hidden-test-cmd` é ignorado se `tests/hidden/` não existe → falso PASS.
- **Evidência**: ~18 repos; explícito em [commander-js](commander-js.md), [express](express.md), [excalidraw](excalidraw.md), [bat](bat.md), etc.
- **Fonte**: SpecBench / ImpossibleBench / achado #2.
- **Mudança**: `HiddenAvailable = cfg.HiddenTest != ""` **ou** dir existe; se cmd setado e falhar ao rodar, refletir no veredicto; documentar.
- **Pacote**: `internal/verify`, `cmd/tb/verify`.
- **Aceite**: sem dir + cmd=`false` → não PASS; com dir vazio + default go hidden → comportamento documentado.
- **Tamanho**: S

#### P0-3 — Exit code de `guard check --strict` com `--json`

- **Problema**: JSON early-return impede `os.Exit(1)` → CI verde com violação.
- **Evidência**: [jq](jq.md), [excalidraw](excalidraw.md), [commander-js](commander-js.md) (quando exercitado).
- **Fonte**: ADR-005; AI Control / monitors.
- **Mudança**: aplicar `strict` **depois** do encode JSON (ou retornar `error`/`ExitError`).
- **Pacote**: `cmd/tb/guard.go`.
- **Aceite**: `--strict --json` com `SeverityError` → exit ≠ 0; stdout continua JSON válido.
- **Tamanho**: XS

#### P0-4 — Honrar `guard.yaml` (paths protegidos configuráveis)

- **Problema**: só hardcoded; custom ignorado em 20/20.
- **Evidência**: 20 confirmados.
- **Fonte**: ADR-005; ImpossibleBench (proteger scoring).
- **Mudança**: carregar `.toolbelt/guard.yaml` (`protected_paths`) e unir aos defaults.
- **Pacote**: `internal/guard`, docs.
- **Aceite**: proteger `lib/help.js` / `src/click/core.py` via YAML gera `protected_path`.
- **Tamanho**: S

#### P1-1 — `tb init` detecta toolchain (não assumir Go)

- **Problema**: `go test ./...` em Python/JS/Rust/C.
- **Evidência**: ~19 repos; pior no grupo sem AI.
- **Fonte**: *Effective harnesses*; AGENTS.md.
- **Mudança**: detectar `go.mod` / `package.json` / `pyproject.toml` / `Cargo.toml` / `Makefile` e preencher `verify.test_cmd` (e sugerir hidden/lint).
- **Pacote**: `internal/config`, `cmd/tb` init.
- **Aceite**: init em fixture npm → `npm test`; python → `pytest`; go inalterado.
- **Tamanho**: M

#### P1-2 — Review: acabar FP da substring `tests/hidden`

- **Problema**: ticket/diff que só *menciona* a regra dispara finding.
- **Evidência**: 16 confirmados ([jq](jq.md), [bat](bat.md), [crush](crush.md), …); alguns refutados quando o diff não citava a string.
- **Fonte**: ADR-010; Compact-Bench (regras importam, mas sinal limpo).
- **Mudança**: casar path alterado via `inScope`/`hitForbidden`, não `Contains` no texto do diff.
- **Pacote**: `internal/review`.
- **Aceite**: `rd.md` com `forbidden: tests/hidden/**` sem tocar o dir → sem finding critical; tocar arquivo → finding.
- **Tamanho**: S

#### P1-3 — Usar `bench.default_agent` quando `--agent` omitido

- **Problema**: campo de config morto; 20/20.
- **Evidência**: checklist unânime.
- **Fonte**: DX / harness.
- **Mudança**: `parseBenchRunFlags` faz fallback para `cfg.Bench.DefaultAgent`.
- **Pacote**: `cmd/tb/bench.go`.
- **Aceite**: YAML com `default_agent: echo hi` + `tb bench run -r 1` não exige flag.
- **Tamanho**: XS

#### P1-4 — Parser de Scope: headings + listas Markdown

- **Problema**: `## Scope` + `- path` falha ou pega só o 1º item.
- **Evidência**: [cobra](cobra.md), [crush](crush.md), relatos de drift “literal” mal interpretados.
- **Fonte**: Claim Plane / CAID (mandate com arquivos).
- **Mudança**: parsear bloco após `## Scope`/`## Escopo`; aceitar bullets; manter globs atuais.
- **Pacote**: `internal/review` (`parseTicketMarkdown`).
- **Aceite**: fixture `rd.md` com heading+bullets → `Scope` completo; `src/**` e `src/*.go` regressão OK.
- **Tamanho**: S

#### P1-5 — Verify: pipeline opcional build/lint (ou rejeitar YAML inerte)

- **Problema**: `lint_cmd`/`build_cmd` no YAML são silenciosos; PASS sem compilar.
- **Evidência**: 20 confirmados “checks parciais”.
- **Fonte**: juízes com sinal externo; SWE-agent (lint no edit).
- **Mudança**: ou (A) honrar `verify.build_cmd`/`lint_cmd` se presentes, ou (B) erro/aviso se chaves desconhecidas; documentar.
- **Pacote**: `internal/config`, `internal/verify`.
- **Aceite**: repo com lint falhando não dá PASS se lint configurado.
- **Tamanho**: M

#### P2-1 — Ler/`seed` de `AGENTS.md` no init/mem/ctx

- **Problema**: grupo com AI: arquivos ignorados; grupo sem: nada gerado.
- **Evidência**: 19+; crush/excalidraw/ky explicitamente.
- **Fonte**: agents.md; Anthropic skills; Cost-Inefficient Behaviors.
- **Mudança**: se existir, indexar/avisar no init; se não, opção `--write-agents` mínimo apontando test/lint/tb.
- **Pacote**: `cmd/tb` init, `internal/mem` (leve).
- **Aceite**: init em repo com `AGENTS.md` registra path; flag gera esqueleto sem sobrescrever.
- **Tamanho**: M

#### P2-2 — Arestas `calls` mínimas para Python (e depois Rust)

- **Problema**: `graph callers`/`impact` vazios onde a tarefa era Py/Rust.
- **Evidência**: click, requests, mcp-python-sdk, pydantic-ai, bat, ripgrep, ruff.
- **Fonte**: LocAgent / RepoGraph; ADR-011 (útil quando localizar é o gargalo).
- **Mudança**: extrator regex conservador de chamadas `name(` resolvendo contra funcs indexadas no mesmo módulo; métrica de precisão em fixture.
- **Pacote**: `internal/graph`.
- **Aceite**: fixture Python com A→B gera ≥1 `EdgeCalls`; callers não vazio.
- **Tamanho**: M

#### P2-3 — Visual report: `skipped=true` se todas as telas foram skipped

- **Problema**: top-level mente; 13 confirmados / 7 refutados (cenários diferentes).
- **Evidência**: bat, crush, excalidraw vs click/typer (só manifesto ausente).
- **Fonte**: Design2Code (métricas honestas).
- **Mudança**: após `runVisualScreens`, se len(screens)>0 e all skipped → `report.Skipped=true`.
- **Pacote**: `internal/verify/visual.go`.
- **Aceite**: manifesto sem references → JSON `skipped: true`.
- **Tamanho**: XS

#### P3-1 — Sincronizar `docs/cli.md` com a CLI real

- **Problema**: `mem add --type`, `tb ticket`, `tb serve`, guard snapshot, etc.
- **Evidência**: 20/20.
- **Fonte**: Writing effective tools (nomes estáveis).
- **Mudança**: docs-only alinhando comandos existentes; marcar Fase 3 como planejado.
- **Pacote**: `docs/cli.md` (+ talvez README status).
- **Aceite**: nenhuma flag documentada que a CLI rejeite nos exemplos “implementados”.
- **Tamanho**: S

### PRs separados (ordem de merge recomendada)

Ordem evita conflitos (graph → verify/guard → review/config → docs) e entrega valor cedo.

| # | Título do PR | Uma linha | Prioridade | Depende de |
|---|--------------|-----------|------------|------------|
| 1 | `fix(graph): não aplicar SkipDirs na raiz do walk` | Corrige grafo vazio em dirs chamados `target`/`build`/`dist`. | P0 | — |
| 2 | `fix(verify): rodar hidden_test_cmd sem exigir tests/hidden/` | Held-out configurado passa a executar de verdade. | P0 | — |
| 3 | `fix(guard): exit ≠ 0 com --strict --json` | CI deixa de engolir violações em JSON. | P0 | — |
| 4 | `feat(guard): carregar protected_paths de guard.yaml` | Paths custom passam a valer. | P0 | 3 (mesmo pacote cmd; merge após exit code) |
| 5 | `feat(init): detectar toolchain e preencher verify.test_cmd` | Para de assumir Go em npm/cargo/pytest. | P1 | — |
| 6 | `fix(review): FP de invariante tests/hidden por path, não substring` | Ticket que cita a regra deixa de ser finding. | P1 | — |
| 7 | `fix(bench): honrar bench.default_agent` | Config deixa de ser campo morto. | P1 | — |
| 8 | `fix(review): parsear ## Scope e listas Markdown` | Mandate em Markdown humano funciona no drift. | P1 | 6 (mesmo parser) |
| 9 | `feat(verify): honrar build_cmd/lint_cmd opcionais` | Falso PASS sem lint/build fica opt-in corrigível. | P1 | 2 |
| 10 | `feat(init): seed/leitura de AGENTS.md` | Liga o tb às skills humanas do repo. | P2 | 5 |
| 11 | `feat(graph): arestas calls básicas para Python` | callers/impact úteis fora de Go/JS. | P2 | 1 |
| 12 | `fix(verify): visual skipped agregado + docs(cli) alinhados` | Honestidade do ruler visual + docs; pode ser 1 PR docs+visual ou 2 micros. | P2/P3 | — |

Máximo 12 PRs; o item 12 pode fatiar em `fix(verify): visual skipped` + `docs(cli): alinhar à CLI` se o review preferir diffs ainda menores.

---

## (f) Explicitamente **não** recomendado (agora)

| Item | Por quê |
|------|---------|
| Remover mode `0600` dos configs | Confirmado 20×, mas é higiene de segredo; não é bug de produto. |
| Ligar `TB_GRAPH=on` por default | ADR-011 + papers de custo; campo mostra utilidade **sob demanda**, não obrigatoriedade. |
| Exigir `--fake-config` no fake agent | Claim **refutada** na maioria dos repos. |
| Restringir drift a paths literais | Globs simples **já funcionam**; o buraco real é o parser de heading. |
| Implementar router/A2A/workers neste ciclo | Pesquisa forte, **zero** evidência de campo neste estudo de bugfix single-agent. |
| Mutação de testes / bench impossível / proxy de rede | Sem exercício de campo; manter no roadmap Fase 3+, não na fila imediata. |
| Reescrever grafo com tree-sitter completo agora | Custo alto; ganho incremental com `calls` regex em Python já ataca a dor observada. |
| Tratar “tb não empurra completar tarefa” só com prompt | Precisa de artefatos (progress/features/verify gate), não de mais texto. |

---

## Próximo passo

Usar esta síntese como **entrada da próxima fase de implementação**, começando pelos PRs P0 na ordem da tabela. Os relatórios brutos permanecem em `docs/campo/<slug>.md` para rastreio de evidência.

**Rodada 2** (revalidação das 12 correções em campo): síntese em [`docs/campo/rodada2/SINTESE.md`](rodada2/SINTESE.md); relatórios em [`docs/campo/rodada2/<slug>.md`](rodada2/).
