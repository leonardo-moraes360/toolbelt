# Síntese do estudo de campo do `tb` — rodada 2 (20 repositórios)

> Data: 2026-11-16 · Branch: `campo2/sintese` · Binário típico dos agentes: `toolbelt@a46c295`  
> Relatórios brutos: [`docs/campo/rodada2/<slug>.md`](.) · Template: [`PROMPT.md`](PROMPT.md) · Rodada 1: [`../SINTESE.md`](../SINTESE.md)

Este documento agrega os 20 relatórios da rodada 2 (revalidação das **12 correções** mergeadas após a rodada 1), sem reimplementar código. Contagens e vereditos vêm **somente** das tabelas e seções dos relatórios citados.

---

## (a) Matriz das 12 correções × 20 repositórios

### Legenda

| Símbolo | Significado (normalizado da coluna de status dos relatórios) |
|---------|--------------------------------------------------------------|
| ✓ | funcionou |
| ~ | parcial / funcionou com ressalva |
| ✗ | não funcionou |
| N/A | não se aplica |
| NT | não testado |

### Correções (mesma ordem do [`PROMPT.md`](PROMPT.md))

1. graph SkipDirs na raiz do walk  
2. verify `hidden_test_cmd` sem `tests/hidden/`  
3. guard `--strict --json` exit ≠ 0  
4. guard lê `protected_paths` de `guard.yaml`  
5. init detecção de toolchain  
6. review `tests/hidden` casa por path  
7. bench honra `default_agent` e `compare --agent`  
8. review/drift parseia `## Scope`/`## Escopo`, listas MD e globs `**`  
9. verify `build_cmd` e `lint_cmd`  
10. init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents`  
11. graph arestas `calls` em Python  
12. verify visual skipped agregado + docs CLI alinhados  

### Matriz

Abreviações de coluna = slug (ordem do estudo).

| # | pm | ol | k6 | ex | ky | mcp | cr | se | ru | pa | co | gi | rg | ba | ty | cl | re | xp | cm | jq | ✓ | ~ | ✗ | N/A |
|---|----|----|----|----|----|-----|----|----|----|----|----|----|----|----|----|----|----|----|----|----|---|---|---|-----|
| 1 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 2 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 3 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 4 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **19** | **1** | 0 | 0 |
| 5 | ✓ | ✓ | ✓ | ~ | ~ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | **16** | **4** | 0 | 0 |
| 6 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **19** | **1** | 0 | 0 |
| 7 | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **19** | **1** | 0 | 0 |
| 8 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **18** | **2** | 0 | 0 |
| 9 | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **20** | 0 | 0 | 0 |
| 10 | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ~ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | **16** | **4** | 0 | 0 |
| 11 | N/A | N/A | N/A | N/A | N/A | ✓ | N/A | ✓ | ✓ | ~ | ✓ | N/A | N/A | N/A | ✓ | ✓ | ✓ | N/A | N/A | N/A | **7** | **1** | 0 | **12** |
| 12 | ~ | ~ | ~ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ~ | ✓ | ✓ | ~ | ✓ | **13** | **7** | 0 | 0 |

Slugs: pm=[playwright-mcp](playwright-mcp.md), ol=[ollama](ollama.md), k6=[k6](k6.md), ex=[excalidraw](excalidraw.md), ky=[ky](ky.md), mcp=[mcp-python-sdk](mcp-python-sdk.md), cr=[crush](crush.md), se=[sentry-python](sentry-python.md), ru=[ruff](ruff.md), pa=[pydantic-ai](pydantic-ai.md), co=[cobra](cobra.md), gi=[gin](gin.md), rg=[ripgrep](ripgrep.md), ba=[bat](bat.md), ty=[typer](typer.md), cl=[click](click.md), re=[requests](requests.md), xp=[express](express.md), cm=[commander-js](commander-js.md), jq=[jq](jq.md).

### Totais por correção (resumo)

| # | Correção | ✓ | ~ | ✗ | N/A | Leitura agregada |
|---|----------|---|---|---|-----|------------------|
| 1 | SkipDirs na raiz | 20 | 0 | 0 | 0 | Consenso unânime; nuance de **symlink de raiz** aparece como problema *novo* em [k6](k6.md)/[click](click.md), não como falha do fix |
| 2 | `hidden_test_cmd` sem dir | 20 | 0 | 0 | 0 | Held-out configurado passa a executar |
| 3 | `--strict --json` exit ≠ 0 | 20 | 0 | 0 | 0 | CI deixa de engolir violação em JSON |
| 4 | `guard.yaml` `protected_paths` | 19 | 1 | 0 | 0 | ~ em [gin](gin.md): `secrets/` (só barra) não casa filhos via `filepath.Match` |
| 5 | init toolchain | 16 | 4 | 0 | 0 | Detecta `npm`/`cargo`/`pytest`/`uv`/`make`/`go`; ~ = não preenche `build_cmd`/`lint_cmd` ou escolhe harness incompleto ([excalidraw](excalidraw.md), [ky](ky.md), [sentry-python](sentry-python.md), [typer](typer.md)) |
| 6 | review path `tests/hidden` | 19 | 1 | 0 | 0 | ~ em [pydantic-ai](pydantic-ai.md): sem FP de substring, mas `forbidden_hit` null ao tocar o dir |
| 7 | `default_agent` / `compare --agent` | 19 | 1 | 0 | 0 | ~ em [playwright-mcp](playwright-mcp.md): agente default OK, mas `tasks_dir` absoluto do YAML ignorado |
| 8 | Scope MD + `**` | 18 | 2 | 0 | 0 | ~ em [pydantic-ai](pydantic-ai.md) (`Forbidden **`) e [gin](gin.md) (frontmatter YAML sem `scope:` ignora `## Escopo` no body) |
| 9 | `build_cmd`/`lint_cmd` | 20 | 0 | 0 | 0 | Honrados quando setados; lacuna residual é **auto-preenchimento** no init |
| 10 | AGENTS/`--write-agents` | 16 | 4 | 0 | 0 | ~ = lê mas hints vazios/ruidosos ou seed incompleto ([ky](ky.md), [crush](crush.md), [ruff](ruff.md), [click](click.md)) |
| 11 | Python `calls` | 7 | 1 | 0 | 12 | N/A fora de Python (12); ✓ nos repos Python (+ probes em [ruff](ruff.md)/[cobra](cobra.md)); ~ em [pydantic-ai](pydantic-ai.md) (`callers redact_secrets` vazio) |
| 12 | visual skipped + docs | 13 | 7 | 0 | 0 | Agregação “todas as telas skipped” OK na maioria; ~ por `screens: []` ainda `skipped=false` e/ou docs CLI residuais |

**Nenhum relatório marcou ✗ (não funcionou) ou NT (não testado) nas 12 correções.** Parciais concentram-se em onboarding profundo (5, 10), edge cases de scope/forbidden (6, 8), bench paths (7), globs de guard (4), callers Python (11) e visual/docs (12).

---

## (b) Comparação com a rodada 1

### Método de comparação

- **R1**: [`docs/campo/SINTESE.md`](../SINTESE.md) + [`docs/campo/<slug>.md`](../) · binário típico `c8aae04`.  
- **R2**: este diretório · binário típico `a46c295`.  
- Veredito por repo = seção **“Comparação com a rodada 1”** de cada relatório R2 (não inventado aqui).  
- Falhas R2 são em grande parte **probes intencionais** (drift OOS, guard strict, flags mortas); na R1 muitas falhas eram **bugs de produto** das 12 correções.

### Tabela resumo

| Slug | Tarefa R1 → R2 | Completa R1 | Completa R2 | Falhas tb R1 → R2 (relato) | Wall / sessão (relato) | Veredito R2 |
|------|----------------|-------------|{-------------|----------------------------|------------------------|-------------|
| [playwright-mcp](playwright-mcp.md) | env docs → AGENTS+cli tests | sim | sim | 14 → 18 (probes) | ~3,4 min → ~2,5 min cmds | **melhor** |
| [ollama](ollama.md) | verbose launch → HumanNumber | sim | sim | 13 → 3+ intencionais | ~6–10 min → ~8–12 min | **melhor** |
| [k6](k6.md) | TagSet parcial → WithField | **parcial** | **sim** | 12 → probes; 0 crashes | ~70 s log → ~4,5 min harness | **melhor** |
| [excalidraw](excalidraw.md) | normalizeRadians → tarefa nova | sim | sim | bugs produto → ESCALATE lint esperado | ~49 s → ~103 s (mais checks) | **melhor** |
| [ky](ky.md) | retry.jitter → tarefa nova | sim | sim | 14 → probes | tb ~40,7 s → ~6,2 s | **melhor** |
| [mcp-python-sdk](mcp-python-sdk.md) | encoding handler → display_name | sim | sim | 25/70 → 9 | ~35,4 s → ~24,7 s | **melhor** |
| [crush](crush.md) | slog capitalização → Capitalize | sim | sim | 10 → 14 probes | ~3 min → ~2,5 min | **melhor** |
| [sentry-python](sentry-python.md) | EventScrubber → streaming | sim | sim | 13 → 17 probes | ~10–12 min → sessão + ~28 s timed | **melhor** |
| [ruff](ruff.md) | FURB open bytes → N801 | sim | sim | bugs produto → impact/callers vazios Rust | ~6–15 min → ~15–25 min | **melhor** |
| [pydantic-ai](pydantic-ai.md) | last_modified → redact/detectors | sim | sim | 26 → 12 | ~57 s → ~74 s invoc. | **melhor** |
| [cobra](cobra.md) | SetErrPrefix → ExactArgs | sim | sim | 19 → 9 | ~3,6 min → ~2,8 min evid. | **melhor** |
| [gin](gin.md) | BindUnmarshaler → ValidateStruct | sim | sim | 11 → 7 | ~5,2 min → ~3,8 min | **melhor** (com gap novo frontmatter) |
| [ripgrep](ripgrep.md) | multilinha `-U` → hyperlink | sim | sim | ~11–16 → ~8–12 | ~15–20 min → ~8–12 min | **melhor** |
| [bat](bat.md) | language/map-syntax → offset | sim | sim | 13 → ~12–14 | ~25–35 min → ~25–40 min | **melhor** |
| [typer](typer.md) | flaky TMPDIR → ZDOTDIR | sim | sim | 20 → 5 intencionais | ~15–20 min → ~15–25 min | **melhor** |
| [click](click.md) | get_error_hint → lazy file | sim | sim | 19 → ~13 | similar (dezenas de min) | **melhor** |
| [requests](requests.md) | digest qop → IPv6 proxies | sim | sim | ≥15 → ~8 | ~25–30 min → ~25–35 min | **melhor** |
| [express](express.md) | subdomains IPv6 → DataView ETag | sim | sim | 22 → 6 | ~20 s cmds → ~19 s | **melhor** |
| [commander-js](commander-js.md) | Help.groupItems → parse args | sim | sim | 23 → 19 | ~5 min 47 s → ~3 min 46 s | **melhor** |
| [jq](jq.md) | literais `23.` → slice assign | sim | sim | 18 → várias esperadas | ~25–30 min → ~20–25 min | **melhor** |

### Leitura agregada R1 → R2

- **Conclusão local**: R1 19/20 (k6 parcial) → R2 **20/20** sim.  
- **Veredito por repo**: **20/20 “melhor”** nas 12 correções exercitáveis; lacunas que permanecem “iguais” são sobretudo MCP incompleto, completeness de review, mode `0600`, `tb ticket` ausente, grafo Rust/C fraco, e empurrar completude da tarefa.  
- **Falhas tb**: o número bruto às vezes sobe (mais probes das 12 correções); o tipo muda — de *produto quebrado* para *falha esperada de probe*. Caminhos felizes (verify/review/drift finais) passam onde R1 falhava.  
- **Grafo `/tmp/target`**: de 0 nós (vários R1) para milhares (R2), ex. [ky](ky.md) 589, [mcp-python-sdk](mcp-python-sdk.md) ~16k, [requests](requests.md) 853, [express](express.md) 452.  
- **Init**: de `go test ./...` default quase universal → `npm test` / `cargo test` / `pytest` / `uv run pytest` / `make check` conforme toolchain.

---

## (c) Problemas que continuam e problemas novos (deduplicados)

Severidade = a mais alta citada nos relatórios para o tema; contagem = quantos relatórios R2 listam o item em “continuam” / “novos” / sugestões com evidência.

### Continuam da rodada 1 (ainda observados)

| # | Problema | Repos (n) | Sev. | Evidência |
|---|----------|-----------|------|-----------|
| C1 | Init detecta `test_cmd` mas **não preenche** `build_cmd`/`lint_cmd` (nem aplica hints AGENTS/package.json/Makefile) | ≥15 (ex.: [excalidraw](excalidraw.md), [ky](ky.md), [mcp-python-sdk](mcp-python-sdk.md), [sentry-python](sentry-python.md), [crush](crush.md), [requests](requests.md), [express](express.md), [commander-js](commander-js.md), [cobra](cobra.md), [bat](bat.md), [typer](typer.md), [ruff](ruff.md), …) | P1–P2 | Pós-init só `test_cmd`; lint/build manuais |
| C2 | Configs `.toolbelt/*` (e às vezes seed `AGENTS.md`) em mode **0600** | ≥12 | P3 | Checklist R1; reconfirmado em vários R2 |
| C3 | MCP sem paridade CLI (`mem`/`verify`/`review`/`drift`/`guard`) | ≥10 ([ky](ky.md), [sentry-python](sentry-python.md), [typer](typer.md), [requests](requests.md), [gin](gin.md), [ripgrep](ripgrep.md), …) | P2–P3 | `tools/list` só graph/version/echo/… |
| C4 | Review **completeness** ruidosa (aceite com teste no diff ainda warning) | ≥8 ([ollama](ollama.md), [crush](crush.md), [express](express.md), [cobra](cobra.md), [bat](bat.md), [jq](jq.md), …) | P2–P3 | warning “critério pouco evidenciado” |
| C5 | `version` → `unknown` sem ldflags | comum | P3 | esperado sem `-ldflags` |
| C6 | Sem `tb ticket` / mandate de 1ª classe | comum | P3 | docs marcam planejado |
| C7 | Grafo Rust/C: `callers`/`impact` vazios ou inúteis | [ruff](ruff.md), [ripgrep](ripgrep.md), [bat](bat.md), [jq](jq.md) (+ menções) | P1–P2 | impact `(vazio)` apesar de índice |
| C8 | Grafo off sem `TB_GRAPH`/`--enable` (MCP incluso) | comum; [k6](k6.md) | P3 | alinhado ADR-011; ainda atrito |
| C9 | Verify só tão bom quanto `test_cmd` (PASS sem lint se `lint_cmd` vazio) | [commander-js](commander-js.md), [express](express.md), [requests](requests.md), … | P2 | opt-in documentado; falso PASS se agente não setar |
| C10 | tb **não empurra** completar a tarefa sozinho | qualitativo, vários | P2–P3 | sem progress/checklist harness |
| C11 | Docs CLI com exemplos/flags residuais (`mem add --type`, `graph index --path`, etc.) | [ollama](ollama.md), [k6](k6.md), [commander-js](commander-js.md), [playwright-mcp](playwright-mcp.md), … | P3 | CLI rejeita; docs ainda citam |
| C12 | Não lê `AI_POLICY.md` / skills `.agents/` | [ripgrep](ripgrep.md), [commander-js](commander-js.md), [ruff](ruff.md), [typer](typer.md), [sentry-python](sentry-python.md) (`agents.toml`) | P3 | fora do radar do init |

### Novos ou reenquadrados na rodada 2

| # | Problema | Repos (n) | Sev. | Evidência |
|---|----------|-----------|------|-----------|
| N1 | `LoadTicket`: frontmatter YAML **sem** `scope:` **ignora** `## Scope`/`## Escopo` no body | 1 ([gin](gin.md)) | **P1** | review `scope-missing` com frontmatter+body MD |
| N2 | `bench.tasks_dir` / `fixtures_dir` / `output_dir` do YAML **ignorados** (default relativo / `-t` vence) | ≥6 ([playwright-mcp](playwright-mcp.md), [crush](crush.md), [excalidraw](excalidraw.md), [sentry-python](sentry-python.md), [gin](gin.md), [bat](bat.md)) | P2 | `open bench/tasks: no such file` com path absoluto no YAML |
| N3 | `guard.yaml` protegendo o **path da própria tarefa** → `tb verify` FAIL com **0 iterações** (não roda build/test) | ≥4 ([ollama](ollama.md), [k6](k6.md), [jq](jq.md), [bat](bat.md)) | P2 | verify aborta em `protected_path` custom |
| N4 | Raiz **symlink** chamada `target` → grafo **0 nós** (`filepath.Walk` não desce) | ≥2 ([k6](k6.md), [click](click.md)) | P2 | `ln -s … /tmp/target` → 0; dir real OK |
| N5 | `screens: []` → top-level `skipped: false` / `screens: null` / PASS | ≥2 ([playwright-mcp](playwright-mcp.md), [mcp-python-sdk](mcp-python-sdk.md)) | P2 | distinto do caso “todas as telas skipped” |
| N6 | Visual `skipped: true` ainda com **verdict PASS** | ≥1 ([ripgrep](ripgrep.md)) | P2 | manifesto ausente → skipped true + PASS |
| N7 | `Forbidden` / globs `**` **não** populam `forbidden_hit` (só `out_of_scope`) | ≥1 ([pydantic-ai](pydantic-ai.md)) | P2 | cheat em `tests/hidden/` sem `forbidden_hit` |
| N8 | `guard check` UX: `--base`/`--head` **unknown**; exige `A..B` posicional | ≥5 ([playwright-mcp](playwright-mcp.md), [ky](ky.md), [pydantic-ai](pydantic-ai.md), [gin](gin.md), [requests](requests.md)) | P2–P3 | inconsistente com verify/review |
| N9 | Hints AGENTS **ruidosos** (fences Go) ou **vazios** apesar de texto útil | ≥4 ([crush](crush.md), [ky](ky.md), [click](click.md), [cobra](cobra.md)) | P2–P3 | `agent_docs.yaml` |
| N10 | Arestas `calls` Python com **alvos espúrios** / `callers` por nome incompleto | ≥3 ([requests](requests.md), [pydantic-ai](pydantic-ai.md), [click](click.md)) | P2 | ex. `setup.py→tests.setup`; `redact_secrets` vazio |
| N11 | Callers/impact **JS** fracos apesar de índice populado | ≥2 ([commander-js](commander-js.md), [ky](ky.md) `#private`) | P2 | `_prepareUserArgs` → vazio |
| N12 | `tb init` escolhe `uv run pytest` **sem** `uv` no PATH | 1 ([typer](typer.md)) | P2 | `uv: command not found` |
| N13 | Bench marca `passed: true` com agente `exit_code: 1` | 1 ([jq](jq.md)) | P2 | fixture Go + agente `false` |
| N14 | `protected_paths: secrets/` (dir) **não** protege filhos | 1 ([gin](gin.md)) | P2 | `secrets/*` funciona; `secrets/` null |
| N15 | Flag global `-c`/`--config` **não** alimenta `config.Load()` no bench | 1 ([ripgrep](ripgrep.md)) | P2 | ainda exige `--agent` |
| N16 | `tb drift` com `HEAD^` → `base_mismatch` (compara string, não SHA) | 1 ([click](click.md)) | P2 | SHA expandido OK |
| N17 | Seed `AGENTS.md` genérico / mode 0600; não cita toolchain detectada | ≥3 ([express](express.md), [jq](jq.md), [ripgrep](ripgrep.md)) | P2–P3 | stub harness |
| N18 | verify `ESCALATE`/`FAIL` pode sair **exit 0** com `--json` (cenário build falho) | 1 ([jq](jq.md)) | P3 | `build_cmd: false` |
| N19 | `rd.md` fora do Escopo → drift/review error se não listado | ≥3 ([playwright-mcp](playwright-mcp.md), [cobra](cobra.md), …) | P3 | ticket path OOS |
| N20 | Docs/CLI: `graph query` ausente (EXIT 0 + help); `review -o` só `--output` | [commander-js](commander-js.md), [k6](k6.md) | P3 | UX inconsistente |

---

## (d) Próximas melhorias (1 PR cada, ordem de merge)

Ordem evita conflitos (review → init/config → verify/guard → bench → graph → mcp → docs) e entrega valor cedo. Arquivos = pacotes **prováveis** (não editados nesta síntese).

| # | Título sugerido do PR | Uma linha | Pri. | Arquivos prováveis | Depende de |
|---|----------------------|-----------|------|--------------------|------------|
| 1 | `fix(review): merge ## Scope do body quando frontmatter não tem scope` | Fecha o P1 de [gin](gin.md): YAML frontmatter sem `scope:` não pode anular headings MD. | P0/P1 | `internal/review` (parser de ticket) | — |
| 2 | `fix(review): popular forbidden_hit para Forbidden/** e invariante hidden` | Alinha drift/review ao toque real em paths proibidos ([pydantic-ai](pydantic-ai.md)). | P1 | `internal/review` | 1 (mesmo parser; merge após) |
| 3 | `feat(init): preencher/sugerir build_cmd e lint_cmd da toolchain` | Fecha C1 (≥15 repos): `package.json` scripts, `pyproject`/Makefile/`golangci`, hints AGENTS curtos. | P1 | `internal/config`, `cmd/tb` init | — |
| 4 | `fix(init): filtrar hints AGENTS e validar binários no PATH` | Hints sem fences longos ([crush](crush.md)); fallback se `uv` ausente ([typer](typer.md)); opcional extrair invariantes. | P1 | init / `internal/mem` leve | 3 |
| 5 | `fix(bench): honrar tasks_dir/fixtures_dir/output_dir do config.yaml` | Paths absolutos do YAML deixam de ser ignorados (N2). | P1 | `cmd/tb/bench.go`, config bench | — |
| 6 | `fix(bench): não marcar passed quando agente exit ≠ 0` | Corrige [jq](jq.md); opcional exit ≠ 0 se `default_agent` 127. | P1 | `internal/bench` / `cmd/tb/bench.go` | 5 (mesmo pacote; após) |
| 7 | `fix(verify): screens vazias e skipped não dão PASS enganoso` | `screens: []` → `skipped=true` ([playwright-mcp](playwright-mcp.md), [mcp-python-sdk](mcp-python-sdk.md)); skipped ≠ PASS ([ripgrep](ripgrep.md)). | P1 | `internal/verify` (visual) | — |
| 8 | `fix(verify/guard): não abortar verify só por protected_paths do mandate` | Modo dev / exceção para paths no Scope (N3: [ollama](ollama.md), [k6](k6.md), [jq](jq.md), [bat](bat.md)). | P1 | `internal/verify`, `internal/guard` | — |
| 9 | `feat(guard): globs de diretório e aliases --base/--head` | `secrets/` → prefixo de filhos ([gin](gin.md)); UX alinhada a verify/review (N8). | P1 | `internal/guard`, `cmd/tb/guard.go` | 8 (mesmo pacote; após) |
| 10 | `fix(graph): EvalSymlinks / aviso em raiz symlink` | Evita 0 nós em `/tmp/target` linkado ([k6](k6.md), [click](click.md)). | P2 | `internal/graph` | — |
| 11 | `fix(graph): callers Python por sufixo + filtro de confiança` | Reduz ruído ([requests](requests.md)) e buracos de nome ([pydantic-ai](pydantic-ai.md)). | P2 | `internal/graph` (index Python) | 10 |
| 12 | `feat(graph): calls/impact Rust mínimos ou erro explícito` | Fecha C7 ([ruff](ruff.md), [ripgrep](ripgrep.md), [bat](bat.md)); JS class methods opcional no mesmo ciclo ou PR irmão. | P2 | `internal/graph` | 10 |
| 13 | `feat(mcp): expor mem/verify/review/drift/guard` | Paridade com o loop de campo (C3). | P2 | MCP server / tools | — |
| 14 | `fix(cli): drift resolve refs git; exit ≠ 0 em ESCALATE JSON` | `HEAD^` ([click](click.md)); exit code verify ([jq](jq.md)). | P2 | `internal/review` drift, `cmd/tb/verify` | — |
| 15 | `docs(cli): remover exemplos mortos + seed AGENTS com toolchain` | `mem add --type`/`--category`, `graph index --path`, `graph query`; seed cita `test_cmd` detectado (C11, N17, N20). | P3 | `docs/cli.md`, init seed | após 3–4 |

Máximo 15 PRs; o item 12 pode fatiar em `feat(graph): Rust calls` + `fix(graph): callers JS class methods` se o review preferir diffs menores. O item 15 pode ser micros docs-only em paralelo.

### Explicitamente **não** priorizado agora

| Item | Por quê (campo R2) |
|------|---------------------|
| Ligar `TB_GRAPH=on` por default | Ainda alinhado ADR-011; dor é MCP/aviso, não obrigatoriedade |
| Remover mode `0600` de configs secretos | Continua higiene; só seed `AGENTS.md`/YAML públicos pedem 0644 |
| Reescrever grafo com tree-sitter completo | Custo alto; ganho incremental com callers/filtro/Rust mínimo ataca a dor |
| Router/A2A/workers | Zero evidência nestas 20 tarefas single-agent |

---

## Próximo passo

Usar esta síntese como **entrada da próxima fila de PRs**, começando pelos itens 1–9 (P0/P1). Relatórios brutos permanecem em `docs/campo/rodada2/<slug>.md`. Síntese da rodada 1: [`docs/campo/SINTESE.md`](../SINTESE.md).
