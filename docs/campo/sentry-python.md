# Campo: getsentry/sentry-python

Estudo de campo do `tb` (build local `make build`, versão com ldflags `c8aae04`) usando o fluxo de agente num clone local de `https://github.com/getsentry/sentry-python`. Sem push, issues, PRs ou comentários no upstream. Evidências em `/tmp/tb-campo-evidence/` nesta VM.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | getsentry/sentry-python |
| Linguagem | Python (SDK) |
| Tamanho | ~19 MiB clone shallow; ~429 MiB após `.venv`/`.tox`; ~523 `.py` fora de venv/tox; GitHub `size` ≈ 119 MiB; ~2220 stars |
| Instrumentação AI existente | `AGENTS.md` (fonte), `CLAUDE.md` → symlink para `AGENTS.md`, `agents.toml` (skills getsentry: code-review, find-bugs, skill-scanner, security-review) |
| Toolchain | **Parcialmente configurada**: `uv` instalado; `uv sync` (dev: tox, tox-uv, ruff, pre-commit); `uv pip install -e .`; tox env `py3.12-common` funcional; `uv run --group typing mypy` ok para `sentry_sdk/scrubber.py`; ruff check/format ok nos arquivos tocados |
| Não configurado | matriz completa tox (integrações Django/Flask/AWS/MCP/agents etc.), Python 3.6–3.11/3.13+/free-threaded, gevent/eventlet, pre-commit hooks instalados no git do alvo, docs Sphinx |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `EventScrubber` para normalizar hífen/underscore nas chaves do denylist e remover `x_forwarded_for` de `DEFAULT_DENYLIST` (fica só no PII). Atualizar teste WSGI legado afetado. |
| Por quê | Bug real, escopo pequeno/médio, com caminho de testes claro; maintainer plausivelmente aceitaria após discussão de produto (label Waiting for: Product Owner). |
| Issue | https://github.com/getsentry/sentry-python/issues/8272 (somente leitura; sem interação) |
| Completa? | **Sim no clone local** (commits locais `b372f0e`, `799876a` na branch `local/campo-scrubber-8272`; **não** enviados ao upstream) |
| Resultado nativo | `TESTPATH=tests/test_scrubber.py uv run tox -e py3.12-common`: **15 passed**. Legacy WSGI após ajuste: **2 passed**. `ruff check`/`ruff format --check` nos arquivos tocados: ok. `mypy sentry_sdk/scrubber.py`: ok. Primeira corrida sem `TESTPATH` (suite common inteira) falhou **1** teste (`test_request_headers_legacy_no_pii_redacts_sensitive`) — esperado pela própria issue; corrigido em seguida |

Arquivos locais alterados: `sentry_sdk/scrubber.py`, `tests/test_scrubber.py`, `tests/integrations/wsgi/test_wsgi.py`.

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; criou `.toolbelt/` + `config.yaml` | atrapalhou | Defaults `verify.test_cmd: go test ./...` / `hidden_test_cmd: go test ./tests/hidden/...` num repo Python; **não** leu `AGENTS.md`/`agents.toml`; permissões `0600` em config |
| `tb mem init/list/add/check` | OK; 3 invariantes padrão + 1 adicionada | neutro | `mem add` só posicional; invariantes genéricas (`tests/hidden`, Go-ish) pouco alinhadas ao AGENTS.md do Sentry |
| Ticket / Scope (`rd.md`) | Manual (não há `tb ticket`) | ajudou (quando usado) | Scope estreito detectou depois o WSGI fora de escopo via drift/review |
| `tb graph --enable rebuild` (cwd `/tmp/target`) | **0 nós / 0 arestas** em ~8 ms | atrapalhou | Root dir chamado `target` ∈ `SkipDirs` → `SkipDir` na raiz |
| `tb graph --enable rebuild sentry_sdk` | 2800–2801 nós, ~4735 arestas (~0,33 s) | ajudou (parcial) | Indexação Python gera `defines`/`imports` apenas |
| `tb graph search EventScrubber` | Achou `class:scrubber.EventScrubber` após index scoped | ajudou | Útil para localizar símbolo |
| `tb graph impact --file …` | `(vazio)` | atrapalhou | 0 arestas `calls` no grafo Python → impact inútil |
| `tb graph callers --function scrub_dict` | `(vazio)` | atrapalhou | Idem (sem `calls`) |
| `tb graph deps --file scrubber.py` | 3 pacotes (`_types`, `utils`, `typing`) | neutro | Só imports |
| `tb drift` (scope sem WSGI) | exit 1; `out_of_scope: tests/integrations/wsgi/test_wsgi.py` | ajudou | Pegou arquivo necessário fora do mandate |
| `tb drift` (scope completo) | exit 0 | ajudou | Base commit bateu |
| `tb review` | Findings só de scope/invariantes heurísticas; sem review semântica de Python | neutro/atrapalhou | Goldfish não avaliou qualidade do fix; FP se diff menciona `tests/hidden` |
| `tb guard check` | Sem violações no diff real; ignore `guard.yaml` | atrapalhou (config) | Paths hardcoded; `tox.ini` em `guard.yaml` **não** flagado |
| `tb verify run` (testes reais) | PASS em ~5,5 s; `hidden_available: false` | atrapalhou (parcial) | `hidden_test_cmd` **só roda** se existir dir `tests/hidden/`; ruff passado como hidden **não executou** |
| `tb verify run --ruler visual` | PASS com skip; top-level `skipped:false` com screens skipped | atrapalhou (UX) | Ver problemas conhecidos |
| `tb bench run --fake` | Roda; feature-small 0% sem arquivos de solução | neutro | `--fake` sem `--fake-config` não aplica patches |
| `tb bench compare --fake` (A/B graph) | OK (~0,12 s) | neutro | Sem flag `--agent` |
| `tb mcp` | `tools/list` OK (version/echo/env/run/graph_*) | neutro | Sem tools de mem/verify/review/drift/guard |
| Checks nativos vs tb | tox/ruff/mypy pegaram regressão WSGI e syntax; tb verify com `test_cmd=true` deu PASS com SyntaxError no código | atrapalhou | Ver seção “O que o tb pegou…” |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --agent …` → `unknown flag: --agent`; help só tem `--fake` / configs A/B / JSONL |
| `guard.yaml` ignored | **confirmado** | `.toolbelt/guard.yaml` listava `tox.ini`; diff que tocava `tox.ini` + thresholds + `tests/hidden` só reportou paths **hardcoded** (`thresholds.yaml`, `tests/hidden/*`), não `tox.ini` |
| Outdated CLI docs | **confirmado** | `docs/cli.md` documenta `tb run`, `tb proc`, `tb md`, `tb ticket`, `tb board`; `tb ticket` → `unknown command`; help real não lista esses |
| Version shows `unknown` | **confirmado** (build sem ldflags) | `go build -o /tmp/tb-nolink` → `commit: unknown`, `built: unknown` (versão `0.1.0-dev`). Com `make build`: `c8aae04` |
| Config files created `0600` | **confirmado** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` todos `600` |
| Visual ruler reports `skipped=false` when skipping | **confirmado** | Com manifesto exemplo e refs ausentes: top-level `"skipped": false` enquanto cada screen tem `"skipped": true`, `passed: true`, verdict PASS |
| `default_agent` ignored | **confirmado** | `config.yaml` com `bench.default_agent: echo-agent {prompt}`; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| Guard aborts verify when diff touches hardcoded config path | **confirmado** | Diff com `.toolbelt/thresholds.yaml` / `tests/hidden/…` → verify exit 1, `iterations: 0`, verdict FAIL sem rodar testes |
| Default fake agent needs `--fake-config` | **confirmado** (para passar tarefas com código) | Sem config: `feature-small` passed=false (não cria/modifica arquivos). Default `ShouldPass:true` só emite JSON. Com tarefa `mcp-smoke` (verificação trivial) passed=true sem config |
| `mem add` positional | **confirmado** | `tb mem add --rule foo` → `unknown flag: --rule`; usage `tb mem add <regra>` |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `tests/**` funciona (caso especial); `tests/test_*.py` via `filepath.Match` funciona no mesmo segmento; docs/UX ainda empurram paths literais |
| `graph impact` needs `--file` | **confirmado** | Sem flag: `--file é obrigatório` |
| Review false positives on `tests/hidden` invariant | **confirmado** | Comentário no diff contendo a string `tests/hidden` (sem tocar o dir) → finding critical `possível violação de invariante: NUNCA modifique arquivos em tests/hidden/` (agreement 3) |
| Verify runs only partial checks / misses build errors | **confirmado** | (1) hidden cmd ignorado sem `tests/hidden/`; (2) `test_cmd=true` + SyntaxError em `scrubber.py` → verify PASS; import nativo falha |
| tb não empurra completar a tarefa inteira | **confirmado (observacional, N=1)** | Após fix do scrubber, suite common ainda falhava no WSGI; tb verify configurado só em `test_scrubber.py` deu PASS; foi o drift de scope (e a issue) que apontaram o gap — não um checklist/harness de completion. Sem A/B controlado neste repo |

## Problemas novos

### P0 — `SkipDirs` inclui `target` e zera o grafo se o clone mora em `…/target`

- **Repro**: `git clone … /tmp/target && cd /tmp/target && tb init && tb graph --enable rebuild .` → `0 nós`. Controle: dir `/tmp/not-target-name` indexa; `/tmp/tb-graph-bug-demo/target` → 0 nós.
- **Esperado**: pular só subdirs chamados `target` (ex. Rust), não a raiz do walk.
- **Atual**: `walkSourceFile` faz `SkipDir` quando `info.Name()` ∈ SkipDirs, inclusive na raiz absoluta.

### P1 — Indexador Python não emite arestas `calls`

- **Repro**: após `tb graph --enable rebuild sentry_sdk`, contagem de edges: `defines`/`imports` apenas; `calls=0`. `tb graph impact --file scrubber.py` e `callers --function scrub_dict` → `(vazio)`.
- **Esperado**: impact/callers úteis em Python (repo do grupo).
- **Atual**: comandos “funcionam” mas sempre vazios → falsa sensação de cobertura.

### P1 — `tb init` não detecta ecossistema / não importa AGENTS.md

- **Repro**: init em sentry-python deixa `verify.test_cmd: go test ./...`.
- **Esperado**: defaults a partir de `pyproject.toml`/`tox.ini`/`AGENTS.md` (tox via uv, ruff, mypy).
- **Atual**: agente que confia no default roda Go num repo Python.

### P2 — `hidden_test_cmd` só executa se existir `tests/hidden/`

- **Repro**: verify com `--hidden-test-cmd 'uv run ruff check …'` num repo sem esse dir → JSON `hidden_available: false`, ruff nunca rodou, verdict ainda PASS se visible passou.
- **Esperado**: comando hidden configurado sempre roda (ou aviso explícito).
- **Atual**: flag silenciosamente no-op.

### P2 — UX de flags vs docs (`graph index --path`, `mem add --rule`)

- **Repro**: `tb graph index --path …` → unknown flag (positional `[path]`); `mem add --rule` → unknown flag.
- **Esperado**: flags documentadas ou erros que mostrem a forma correta sem parecer bug.

### P3 — MCP não expõe mem/verify/review/drift/guard

- **Repro**: `tools/list` → só `tb_version`, `tb_echo`, `tb_env`, `tb_run`, graph_*.
- **Esperado**: paridade com o CLI usado no fluxo de campo (grupo com instrumentação de agente).

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift/review: `tests/integrations/wsgi/test_wsgi.py` fora do scope inicial.
- Guard: alteração a `.toolbelt/thresholds.yaml` e `tests/hidden/*`.
- Graph search: localizou `EventScrubber` após indexar `sentry_sdk`.
- Verify: falhou (ESCALATE) quando o `test_cmd` real importava SDK com SyntaxError.

**Deixou passar / não ajudou**
- Regressão do teste WSGI enquanto verify estava limitado a `tests/test_scrubber.py` (PASS enganoso para a tarefa completa).
- SyntaxError com `test_cmd=true` → PASS.
- `hidden_test_cmd` (ruff) não executado.
- Impact/callers vazios em Python.
- Review não criticou o design do patch; só heuristics de scope/invariantes.
- Init/defaults Go; zero aproveitamento de AGENTS.md.
- Grafo full rebuild inútil em `/tmp/target`.

## Interação com instrumentação existente / onboarding

| Artefato do repo | O que o tb fez |
|------------------|----------------|
| `AGENTS.md` / `CLAUDE.md` | **Ignorou**. Não importou regras (tox via uv, ruff, mypy, não editar `tox.ini` gerado, guidelines de integração/teste). |
| `agents.toml` (skills Sentry) | **Ignorou**. Sem ponte para skills MCP/CLI. |
| Onboarding `tb init` | Criou vault/logs/bench vazios + config genérica. **Não** criou progress log / feature list estilo harness; **não** gerou nem mesclou AGENTS.md. |
| Conflito | Invariantes padrão do `tb mem` falam de `tests/hidden/` e thresholds Go-centric; AGENTS.md fala de tox/ruff/mypy e arquivos gerados — sinais paralelos sem fusão. |
| O que o tb deveria importar | Comandos de verify (tox common, ruff, mypy); proibição de editar `tox.ini`/workflows gerados; preferência por testes e2e/`capture_events`; stack `uv run`; skills names de `agents.toml` como hooks opcionais de review |

Achado de pesquisa (`docs/pesquisa/achados.md`: “AGENTS.md como contexto local”) permanece **pendente** na prática.

## Sugestões de melhoria

1. **[P0][graph]** Não aplicar `SkipDirs` ao diretório raiz do walk; renomear skip `target` → padrões mais específicos (`**/target/release`, Cargo) — área `graph`.
2. **[P1][graph]** Extrair arestas `calls` (ou fallback de impact por `imports`/referências) no indexador Python — área `graph`.
3. **[P1][init/mem]** `tb init` detectar Python e popular `verify.*` + opcionalmente importar bullets de `AGENTS.md` para `invariants.yaml` / ticket template — áreas `init`, `mem/ctx`.
4. **[P1][guard]** Carregar `.toolbelt/guard.yaml` de verdade; documentar divergência atual — área `guard`.
5. **[P1][verify]** Rodar `hidden_test_cmd` sempre que configurado; defaults multi-comando (test + lint + typecheck); falhar se só `true`/`go test` em repo não-Go — área `verify`.
6. **[P2][verify/visual]** Propagar `skipped: true` no relatório agregando screens skipped; não marcar `passed: true` sem medição — área `verify`.
7. **[P2][bench]** Honrar `bench.default_agent`; aceitar `--agent` em `compare` A/B; documentar que `--fake` sem `--fake-config` não aplica diffs — área `bench`.
8. **[P2][docs/cli]** Sincronizar `docs/cli.md` com comandos reais ou marcar “planejado”; alinhar flags (`index [path]`, `mem add` posicional) — área `docs`.
9. **[P2][review]** Heurística de invariantes por path tocado, não substring no diff — área `review`.
10. **[P3][mcp]** Expor drift/review/verify/mem no MCP para repos já instrumentados com agentes — área `mcp`.
11. **[P3][router/harness]** Checklist de completion (acceptance do ticket ↔ arquivos/testes) para empurrar a tarefa inteira — área `verify`/`ticket`.

## Métricas

| Métrica | Valor |
|---------|-------|
| Wall time (evidências timed + tox) | ~10–12 min de sessão ativa; span de arquivos de evidência ≈ **577 s**; maior custo = tox common completo **~269 s** (1 fail WSGI) + re-runs focados |
| Invocações `tb` registradas | **38** linhas em `summary.tsv` (+ builds/`mcp`/`version` nolink fora da TSV) |
| Falhas de exit≠0 | **13** no log (maioria probes esperados: drift scope, flags inválidas, verify abort, visual sem manifest, default_agent, etc.); **0 crashes** (panic/segfault) observados |
| Build `tb` | `make build` OK (~8 s incl. download módulos) |
| MCP | `tools/list` OK; 8 tools |
| Commits locais no alvo | 2 de produto (+ probes revertidos); **0** push |
| Branch deste relatório | `campo/sentry-python` (somente `docs/campo/sentry-python.md`) |
