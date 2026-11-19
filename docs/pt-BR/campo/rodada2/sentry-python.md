[🇺🇸 English](../../../campo/rodada2/sentry-python.md) | 🇧🇷 Português

# Campo rodada 2: getsentry/sentry-python

Estudo de campo ROUND 2 do `tb` (build local `make build`, ldflags `a46c295`) num clone local de `https://github.com/getsentry/sentry-python`. Grupo: **com instrumentação AI existente**. Sem push, issues, PRs ou comentários no upstream. Evidências em `/tmp/tb-campo2-evidence/` nesta VM.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | getsentry/sentry-python |
| Linguagem | Python (SDK) |
| Tamanho | ~19 MiB clone shallow; ~472 MiB com `.venv`/`.tox`; ~523 `.py` fora de venv/tox; GitHub `size` ≈ 123 MiB; ~2220 stars |
| Instrumentação AI existente | `AGENTS.md` (fonte), `CLAUDE.md` → symlink para `AGENTS.md`, `agents.toml` (skills getsentry) |
| Toolchain | **Parcialmente configurada**: `uv` 0.13.8; `uv sync` (dev: tox, tox-uv, ruff, pre-commit); `uv pip install -e .`; tox env `py3.12-google_genai-v1.29.0` funcional; `uv run ruff` ok; `uv run --group typing mypy` ok no arquivo tocado |
| Não configurado | matriz tox completa (Django/Flask/AWS/MCP/agents etc.), Python 3.6–3.11/3.13+/free-threaded, gevent/eventlet, pre-commit hooks no git do alvo, docs Sphinx |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Em `accumulate_streaming_response` (google_genai), popular `response_id` / `model` a partir de `chunk.response_id` e `chunk.model_version`; asserções em `test_streaming_generate_content` para `GEN_AI_RESPONSE_ID` / `GEN_AI_RESPONSE_MODEL`. |
| Por quê | Bug real, escopo pequeno, caminho de teste claro; distinto da rodada 1. |
| Issue | https://github.com/getsentry/sentry-python/issues/5812 (somente leitura; sem interação) |
| Completa? | **Sim no clone local** (commit `e8b58b5` na branch `local/campo2-google-genai-5812`; **não** enviado ao upstream) |
| Resultado nativo | `TESTPATH=…/test_google_genai.py uv run tox -e py3.12-google_genai-v1.29.0 -- -k 'test_streaming_generate_content and not data_collection and not tools and not message'`: **4 passed**. `ruff check` / `ruff format --check` nos tocados: ok. `mypy` no `streaming.py`: ok. |
| Rodada 1 (comparação) | Rodada 1: `EventScrubber` hífen/underscore + denylist (`issues/8272`). Esta rodada: streaming google_genai `response_id`/`model_version` (`issues/5812`). |

Arquivos locais alterados: `sentry_sdk/integrations/google_genai/streaming.py`, `tests/integrations/google_genai/test_google_genai.py`.

## Verificação das 12 correções

| # | correção | status | evidência |
|---|----------|--------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph --enable rebuild .` em `/tmp/target` → `8053 nós, 25307 arestas` (antes: 0 nós). Full rebuild no clone de trabalho → 8072 nós / 26373 arestas. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; `tb verify run` com `hidden_test_cmd=uv run ruff check …` → `hidden_available: true`, `hidden_passed: true`. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff tocando `tox.ini`: `tb guard check --json --strict BASE..HEAD` → exit **1** + JSON com `violations`. Sem `--strict`: exit 0 com as mesmas violations. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `tox.ini`; violação `protected_path` em `tox.ini` reportada. Defaults (`thresholds.yaml`) continuam ativos. |
| 5 | init detecção de toolchain | **funcionou** (parcial) | `tb init` → `verify.test_cmd: uv run pytest` (não mais `go test`). Ainda **não** escolhe `uv run tox` preferido pelo `AGENTS.md`. |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket citando `tests/hidden/` sem tocar o dir → `findings: []` (sem FP de substring). |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Sem `--agent`/`--fake`, com `bench.default_agent: "true"` em `.toolbelt/config.yaml` → run OK (`agent=external`). Sem default → erro pedindo `--agent`/`--fake`/`default_agent`. `tb bench compare` aceita `-a/--agent` + `--config-a/--config-b`. |
| 8 | review/drift `## Scope`/`## Escopo`, listas, `**` | **funcionou** | Ticket com `## Escopo` + bullets + `tests/integrations/google_genai/**/*.py` → drift OK. Scope só com `streaming.py` → flagrou o teste (`out_of_scope`). |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | JSON: `build_ran: true`, `build_passed: true`, `lint_ran: true`, `lint_passed: true` + visible/hidden. Build falho (import google-genai ausente no `.venv` raiz) abortou antes dos testes (`step: build_or_lint`) — comportamento desejado. |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** | Criou `.toolbelt/agent_docs.yaml` (`sources: AGENTS.md, CLAUDE.md` + hints tox/ruff/mypy). `tb init --write-agents` em dir vazio gera esqueleto; no repo existente **não** sobrescreve `AGENTS.md` (diff vazio). |
| 11 | graph Python arestas `calls` | **funcionou** | Stats: `edges_calls: 12793` (full) / `4882` (só `sentry_sdk`). `tb graph callers --function accumulate_streaming_response` → `new_iterator` / `new_async_iterator`. `impact --file …/streaming.py` lista wrappers. |
| 12 | verify visual skipped agregado + docs CLI | **funcionou** | Visual sem manifesto: `skipped: true`. Com screens e refs ausentes: top-level `skipped: true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` marca `tb ticket`/`tb run`/etc. como planejados e documenta Scope/hidden/guard.yaml/default_agent. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; `.toolbelt/` + `config.yaml` + `agent_docs.yaml` | ajudou | Detectou Python/`uv`; leu AGENTS/CLAUDE; `test_cmd=uv run pytest` (melhor que Go; ainda subótimo vs tox) |
| `tb init --write-agents` | OK | ajudou | Semear AGENTS em repo vazio; no sentry-python não sobrescreve |
| `tb mem init/list/add/check` | OK | neutro | 3 invariantes padrão + 1 (`tox.ini`); ainda pouco ligadas ao fluxo nativo além do add manual |
| Ticket / `## Escopo` (`rd.md`) | Manual | ajudou | Parser Markdown+globs funcionou no drift/review |
| `tb graph --enable rebuild .` em `/tmp/target` | 8k+ nós | ajudou | SkipDirs na raiz corrigido |
| `tb graph search accumulate_streaming_response` | Achou símbolo | ajudou | Localização rápida |
| `tb graph callers --function …` | 2 callers | ajudou | `calls` Python úteis (antes vazio) |
| `tb graph impact --file …` | Lista impacto | ajudou (parcial) | Vazio se o índice foi feito com cwd/`path` cujo prefixo de arquivo não bate com `--file` (ex.: rebuild `sentry_sdk` + path com `sentry_sdk/…`) |
| `tb graph deps --file …` | 9 pacotes | ajudou | Após rebuild alinhado ao path |
| `tb drift` | exit 0 no scope completo; exit 1 no estreito | ajudou | Pegou teste fora do mandate |
| `tb review` | `findings: []` no patch limpo | neutro | Sem review semântica de Python; heuristics de scope/invariantes OK |
| `tb guard check … --json --strict` | exit 1 com `tox.ini` / thresholds | ajudou | `guard.yaml` + defaults |
| `tb verify run` | PASS com build/lint/test/hidden | ajudou | Rodou tox streaming + ruff; falhou cedo com build_cmd inválido |
| `tb verify run --ruler visual` | PASS skipped | neutro | Agregação `skipped` correta; sem sinal de produto neste SDK |
| `tb bench run` (default_agent) | OK com tasks do toolbelt | neutro | Não há `bench/tasks` no alvo; precisa `-t` externo ou copiar fixtures |
| `tb bench compare --agent` | OK A/B graph-off/on | neutro | Flag `--agent` presente e usável |
| `tb mcp` | `tools/list` OK | neutro | version/echo/env/run/graph_*; sem mem/verify/review/drift/guard |
| Checks nativos vs tb | tox 4 passed; ruff/mypy ok | ajudou | tb verify espelhou o resultado nativo quando `test_cmd` apontou para tox |

## Problemas que continuam

Da rodada 1 (`docs/campo/sentry-python.md`) que **ainda** se observam:

| problema rodada 1 | status rodada 2 |
|-------------------|-----------------|
| Init/defaults pouco alinhados ao `AGENTS.md` (tox via uv) | **continua** (melhorou: não é mais `go test`; ainda `uv run pytest` em vez de tox) |
| `agents.toml` ignorado | **continua** |
| MCP sem mem/verify/review/drift/guard | **continua** |
| Review sem análise semântica do patch Python | **continua** (só scope/invariantes) |
| Config files mode `0600` | **continua** (`config.yaml`/`agent_docs.yaml`/`graph.json` 600) |
| Version `unknown` sem ldflags | **não retestado** com `go build` cru; com `make build` ok (`a46c295`) |
| Fake/`bench` sem tasks no repo alvo | **continua** (precisa apontar para fixtures externas) |
| tb não “empurra” completude da tarefa sozinho | **parcialmente mitigado** por drift de scope; ainda sem harness de acceptance checklist |

Problemas da rodada 1 **corrigidos** neste repo: SkipDirs/`/tmp/target`, `calls` Python, `hidden_test_cmd` gated, `guard.yaml` ignorado, `--strict --json` exit 0, `default_agent` morto, parser `## Scope`, FP `tests/hidden` por substring, verify sem build/lint, visual `skipped=false`, docs CLI desatualizados (planejados marcados), init sem ler AGENTS.

## Problemas novos

### P2 — `graph impact`/`deps` dependem do path exatamente como indexado

- **Repro**: `tb graph --enable rebuild sentry_sdk` depois `impact --file sentry_sdk/integrations/google_genai/streaming.py` → `(vazio)`. Com `--file integrations/google_genai/streaming.py` (ou rebuild `.` + path completo) → resultado útil.
- **Esperado**: resolução tolerante a prefixo de pacote / cwd.
- **Atual**: matching literal do path no índice → falsa sensação de “sem impacto”.

### P2 — `verify.test_cmd` detectado não segue o harness documentado em AGENTS.md

- **Repro**: `tb init` em sentry-python → `uv run pytest`; AGENTS.md manda `uv run tox -e …`.
- **Esperado**: preferir tox quando `tox.ini` + texto AGENTS mencionam tox.
- **Atual**: heurística pyproject/uv → pytest; hints vão para `agent_docs.yaml` mas não viram `test_cmd`.

### P3 — `bench.tasks_dir` no YAML absoluto pode ser ignorado / defaults relativos

- **Repro**: gravar `bench.tasks_dir: /agent/repos/toolbelt/bench/tasks` no config do alvo; `tb bench run -r 1` ainda tentou `bench/tasks` relativo até passar `-t` na CLI.
- **Esperado**: honrar paths absolutos do YAML.
- **Atual**: agente precisa descobrir a flag `-t` (docs ajudam, mas o YAML sozinho falhou neste setup).

### P3 — MCP ainda sem paridade com o fluxo de campo

- **Repro**: `tools/list` após init — sem tools de mem/verify/review/drift/guard.
- **Esperado**: pelo menos verify/drift/guard para repos já instrumentados.
- **Atual**: só utilitários + graph_*.

Nenhuma regressão P0 observada nas 12 correções neste exercício.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift: teste fora do `## Escopo` estreito.
- Guard: alteração a `tox.ini` via `guard.yaml` e a `.toolbelt/thresholds.yaml`.
- Graph: search + callers de `accumulate_streaming_response`; impact após path alinhado.
- Verify: build/lint/hidden/test; falha precoce quando `build_cmd` importava integração sem `google-genai` no venv raiz.
- Review: sem FP ao mencionar `tests/hidden` no ticket.

**Deixou passar / não ajudou**
- Não sugeriu o `test_cmd` tox “certo” a partir do AGENTS.md (agente teve de configurar).
- Review não criticou design/completude do patch além do scope.
- Sem tasks de bench no alvo, benchmark só funciona com fixtures externas.
- `agents.toml` skills não entraram no fluxo.

## Interação com instrumentação existente / onboarding

| Artefato do repo | O que o tb fez |
|------------------|----------------|
| `AGENTS.md` / `CLAUDE.md` | **Leu** → `.toolbelt/agent_docs.yaml` com sources + hints (tox, ruff, mypy, exemplos). **Não** promoveu hints a `verify.test_cmd` automaticamente. `--write-agents` não sobrescreve. |
| `agents.toml` | **Ignorou**. |
| Onboarding `tb init` | Criou vault/logs/bench + config com toolchain Python/`uv`. Melhor que rodada 1; ainda há gap tox vs pytest. |
| Conflito | Invariantes padrão ainda falam de `tests/hidden`/thresholds; AGENTS fala de tox/ruff/mypy — agora convivem com dicas registradas, mas sem fusão automática em `verify.*`. |

## Comparação com a rodada 1

**Melhor**, de forma concreta:

| Área | Rodada 1 | Rodada 2 |
|------|----------|----------|
| Grafo em `/tmp/target` | 0 nós | milhares de nós |
| `calls` Python / callers | vazio | callers e impact úteis |
| `hidden_test_cmd` | no-op sem `tests/hidden/` | executa (ruff rodou) |
| `guard.yaml` | ignorado | `tox.ini` flagado |
| `--strict --json` | exit 0 com violação | exit 1 |
| Init | `go test` + AGENTS ignorado | `uv run pytest` + `agent_docs.yaml` |
| Scope Markdown | frágil / paths literais | `## Escopo` + `**` OK |
| Verify build/lint | inexistente na prática | `build_ran`/`lint_ran` |
| Visual skipped | top-level `skipped=false` | `skipped=true` agregado |
| Docs CLI | comandos fantasmas | marcados como planejados |

Mesmo nível: MCP incompleto; review pouco semântico; bench depende de fixtures externas; `agents.toml` fora do radar.

## Sugestões de melhoria

1. **[P1][init/verify]** Mapear hints de `AGENTS.md`/`tox.ini` → `verify.test_cmd`/`lint_cmd` (ex.: se tox.ini existe e AGENTS cita `uv run tox`, preferir isso a `uv run pytest`) — áreas `init`, `verify`.
2. **[P2][graph]** Resolver `--file` com sufixo/prefixo flexível (aceitar `sentry_sdk/…` e `…` relativo ao subdir indexado) — área `graph`.
3. **[P2][bench]** Honrar `tasks_dir`/`fixtures_dir` absolutos do YAML; erro claro se dir ausente — área `bench`.
4. **[P2][init/mem]** Opcional: importar bullets de AGENTS para invariantes (além de `agent_docs.yaml`) — áreas `init`, `mem`.
5. **[P3][mcp]** Expor drift/review/verify/guard/mem no MCP — área `mcp`.
6. **[P3][init]** Ler `agents.toml` (nomes de skills) como hooks opcionais de review — área `init`/`review`.

## Métricas

| Métrica | Rodada 2 | Rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (soma comandos timed em `summary.tsv`) | ~28 s de `tb`/probes registrados; sessão total maior (~inclui tox ~6 s/run + mypy ~10 s + setup uv/tox) | ~10–12 min sessão; span evidências ≈ 577 s |
| Invocações registradas | **60** linhas em `summary.tsv` (+ mcp/version fora) | **38** |
| Falhas exit≠0 | **17** (probes esperados: drift narrow, guard strict, flags erradas, bench sem tasks/agent, ruff format antes do fix) | **13** |
| Crashes (panic/segfault) | **0** | **0** |
| Build `tb` | `make build` OK (`a46c295`) | `make build` OK (`c8aae04`) |
| MCP tools | 8 (version/echo/env/run/graph_*) | 8 |
| Commits locais no alvo | 1 de produto (+ probes em branch descartável); **0** push | 2; **0** push |
| Branch deste relatório | `campo2/sentry-python` (somente `docs/campo/rodada2/sentry-python.md`) | `campo/sentry-python` |
