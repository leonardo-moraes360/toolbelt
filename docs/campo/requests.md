🇧🇷 Português | [🇺🇸 English](../en/campo/requests.md)

# Estudo de campo: psf/requests

Grupo: sem instrumentação de agente de IA (sem AGENTS.md / CLAUDE.md / .cursorrules).  
Ferramenta: `tb` compilado deste repo (`make build`, versão com ldflags `c8aae04`).  
Clone local: `/tmp/psf-requests` (inicialmente `/tmp/target`; movido após bug de SkipDirs — ver Problemas novos).  
Nenhuma interação com o remoto `psf/requests` (sem push/issue/PR/comentário).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [psf/requests](https://github.com/psf/requests) |
| Linguagem | Python ≥3.10 |
| Tamanho | ~34 `.py` em `src/`+`tests/`; ~6394 LOC em `src/requests`; working tree ~5 MiB (sem `.git`/`.venv`); shallow clone depth 50 |
| Commit base | `3c7baaa` |
| Instrumentação de IA existente | **Não** há `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, prompts de agente. Há `.github/AI_POLICY.md` (política humana anti-slop / copyright — não é harness de agente). |
| Toolchain | **Parcialmente ok.** `python3.12` + venv + `requirements-dev.txt` + `ruff` + `pytest`. `make test` / `pytest tests/` rodou: **615 passed, 4 failed, 15 skipped, 1 xfailed** (~332 s). Falhas: `TestTimeout` contra `10.255.255.1` (timeouts de rede do ambiente, não do código). **Não configurado:** `tox` multi-versão, `pyright`, docs Sphinx, pre-commit hooks completos. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `HTTPDigestAuth.build_digest_header` para selecionar `qop=auth` quando a lista do servidor tem espaços após vírgulas (ex.: `"auth-int, auth"`), via `option.strip()` em cada token. Teste de regressão em `tests/test_lowlevel.py`. |
| Por quê | Bug claro, repro local sem rede, escopo pequeno, plausível para maintainer; issue recente com repro mínimo. |
| Issue | https://github.com/psf/requests/issues/7648 |
| Completada? | **Sim no escopo local do código/teste.** Diff: 1 linha em `auth.py` + teste. **Não** preparado como contribuição upstream completa (sem `HISTORY.md`, sem PR — proibido neste estudo). `rd.md` do tb ficou no commit local do clone-alvo. |
| Checks nativos | `pytest tests/test_lowlevel.py -k digest`: **4 passed**. `ruff check` nos arquivos tocados: **ok**. Suite completa não reexecutada após o fix (baseline pré-fix já tinha 4 falhas de timeout de rede). |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; criou `.toolbelt/` + `config.yaml` 0600 | **Atrapalhou** (defaults Go) | `verify.test_cmd: go test ./...` num repo Python; nada de AGENTS.md; wall ~0,006 s |
| `tb mem init` / `list` / `check` | OK; 3 invariantes anti-cheat padrão | Neutro/ajudou | Arquivo `.toolbelt/invariants.yaml` 0600; `compactacao_segura: true` |
| `tb mem add --type …` (estilo docs) | **Erro** `unknown flag: --type` (exit 1) | **Atrapalhou** | Docs (`docs/cli.md`) desatualizados vs CLI |
| `tb mem add "<regra>" --category mandate` | OK | Ajudou | Posicional obrigatório; wall ~0,006 s |
| `tb graph index` em `/tmp/target` | 0 nós / 0 arestas | **Atrapalhou** | Nome do dir `target` ∈ SkipDirs → SkipDir na raiz |
| `tb graph index .` em `/tmp/psf-requests` | 854 nós, 1179 arestas | Ajudou | wall ~0,074 s após mover o clone |
| `tb graph impact` sem `--file` | Erro obrigatório | Neutro (API rígida) | Confirma issue conhecida |
| `tb graph impact --file src/requests/auth.py` | `(vazio)` | **Atrapalhou** | Não apontou testes/callers; search achou o símbolo |
| `tb graph search build_digest_header` | Achou `func:…build_digest_header` | Ajudou | Útil após index |
| `tb graph callers --function build_digest_header` | `(vazio)` | Atrapalhou | Indexer regex não ligou calls |
| Ticket `rd.md` + Scope | Criado manualmente (YAML frontmatter) | Neutro | tb não gera ticket; Scope literal ajudou drift |
| `tb verify run` com pytest digest | **PASS** (visível true; oculto indisponível) | Ajudou (parcial) | `hidden_available: false` sem dir `tests/hidden/` |
| `tb verify` com default `go test` | **ESCALATE** 3 iterações | **Atrapalhou** | Defaults de init inadequados |
| `tb verify` com `test_utils.py` quebrado + cmd digest | **PASS** | **Atrapalhou** | Miss de erro fora do `test_cmd` |
| `tb verify --ruler visual` | PASS com skip | Neutro | JSON: `"skipped": true`, motivo manifesto ausente |
| `tb review` | Findings bloqueantes | **Atrapalhou** | FP invariante `tests/hidden` + drift de `rd.md` fora do scope inicial |
| `tb drift` | Detectou `rd.md` fora do scope | Ajudou | Comportamento correto; globs `/**` ok, `**/*.py` incompleto |
| `tb guard check` | exit 0 no diff do fix | Neutro | Não leu `.toolbelt/guard.yaml` custom |
| `tb guard` + touch `.toolbelt/thresholds.yaml` via verify | verify **FAIL** iter 0 | Ajudou (anti-cheat) | Path hardcoded aborta verify |
| `tb bench run` sem `--agent` | Erro obrigatório | Neutro | Ignorou `default_agent` no config |
| `tb bench run --fake` | OK em `bugfix-go` | Neutro | Não exigiu `--fake-config` |
| `tb bench compare` (configs A/B) | OK com `--fake` implícito via YAML? / sem flag `--agent` | Neutro/confirmou gap | Help sem `--agent`; compare config-a/b rodou agent=external |
| `tb mcp` + `tools/list` | OK JSON-RPC stdio | Ajudou (smoke) | Tools: version/echo/env/run/graph_* — sem mem/verify/review |
| `tb workspace create` | OK worktree | Neutro | Criou `.toolbelt/workspaces/campo-ws` |
| `tb version` (make build) | `c8aae04` | Neutro | Com ldflags ok |
| `tb` build sem ldflags | `0.1.0-dev`, commit/built **unknown** | Neutro | Confirma issue de versioning |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **Confirmado** | `tb bench compare --help` não lista `--agent`; só `--fake` / JSONL / configs |
| `guard.yaml` ignored | **Confirmado** | `.toolbelt/guard.yaml` protegendo `auth.py`; `tb guard check` exit 0; paths só hardcoded em `internal/guard/guard.go` |
| Outdated CLI docs | **Confirmado** | `docs/cli.md` mostra `tb mem add --type fact`; CLI: `tb mem add <regra>`. README ainda cita `tb md (TUI)` |
| Version shows `unknown` | **Confirmado** (commit/built) | Build sem ldflags: `commit: unknown`, `built: unknown` (versão `0.1.0-dev`) |
| Config files created 0600 | **Confirmado** | `config.yaml`, `invariants.yaml`, `graph.json` → mode `600` |
| Visual ruler reports `skipped=false` when skipping | **Refutado** | `/tmp/verify-visual.json`: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"` |
| `default_agent` ignored | **Confirmado** | Config com `default_agent: "echo-agent {prompt}"`; `tb bench run` ainda exige `--agent` ou `--fake` |
| Guard aborts verify when diff touches hardcoded config path | **Confirmado** | Diff em `.toolbelt/thresholds.yaml` → verify FAIL, violation `protected_path`, iterations 0 |
| Default fake agent needs `--fake-config` | **Refutado** | `tb bench run --fake --task bugfix-go` passou sem `--fake-config` (usa `FakeAgentCommand("")` + default behavior) |
| `mem add` positional | **Confirmado** | CLI: `tb mem add <regra>`; flags `--type/--title` dos docs falham |
| Drift/Scope need literal paths (no globs) | **Parcialmente refutado** | `tests/**` e `src/requests/*.py` funcionam; `**/*.py` **não** é recursivo (`filepath.Match`) — `src/requests/auth.py` não casa, `tests/test_lowlevel.py` casa |
| `graph impact` needs `--file` | **Confirmado** | Sem flag: `Error: --file é obrigatório` |
| Review false positives on tests/hidden invariant | **Confirmado** | Mesmo sem tocar `tests/hidden/`, finding critical: menção em `rd.md` (forbidden) entra no diff e dispara `suggestsViolation` |
| Verify runs only partial checks / misses build errors | **Confirmado** | (1) default `go test` no Python; (2) com `test_cmd` digest, syntax error em `tests/test_utils.py` → verify PASS, pytest nativo ERROR; (3) `hidden_available` só se existir dir `tests/hidden/`, mesmo com `--hidden-test-cmd` |
| tb does not push agent toward completing whole task | **Confirmado** (qualitativo) | Nenhum nudge para HISTORY.md / CONTRIBUTING / checklist de release; review bloqueou em FP em vez de guiar fechamento; graph impact vazio não sugeriu testes; init não detectou pytest/ruff |

## Problemas novos

### P0 — `SkipDirs` inclui o nome do diretório raiz do repo

- **Repro:** `git clone … /tmp/target && cd /tmp/target && tb init && tb graph index --enable .` → `0 nós`. Symlink `/tmp/skipdirs-repro/target` → idem. Após `mv` para `/tmp/psf-requests` → 854 nós.
- **Expected:** Pular só subdirs chamados `target` (ex. Rust), não a raiz do walk.
- **Actual:** `info.Name() == "target"` na raiz → `filepath.SkipDir` em tudo.

### P1 — `hidden_available` amarrado à existência de `tests/hidden/`

- **Repro:** `tb verify run --hidden-test-cmd '…pytest…'` sem criar `tests/hidden/` → `"hidden_available": false`, comando oculto não roda.
- **Expected:** Honrar `--hidden-test-cmd` / config mesmo sem o diretório convencional.
- **Actual:** `os.Stat(tests/hidden)` decide disponibilidade (`internal/verify/verify.go`).

### P1 — Onboarding assume Go

- **Repro:** `tb init` em requests → `verify.test_cmd: go test ./...`.
- **Expected:** Detectar pytest/ruff/tox via `pyproject.toml` / `Makefile`, ou perguntar.
- **Actual:** Defaults fixos Go; agente precisa editar YAML à mão.

### P2 — `graph impact` / `callers` vazios em Python útil

- **Repro:** Após index 854 nós, `impact --file src/requests/auth.py` e `callers --function build_digest_header` → `(vazio)`.
- **Expected:** Pelo menos edges de define/import úteis para impacto.
- **Actual:** Search acha o símbolo; impacto/callers não ajudam a tarefa.

### P2 — UX inconsistente de flags curtas

- **Repro:** `tb verify run -o …` ok; `tb review -o` / `tb drift -o` → `unknown shorthand flag: 'o'`.
- **Expected:** `-o` / `--output` alinhados.
- **Actual:** Só `--output` em review/drift.

### P3 — MCP expõe só subset

- **Repro:** `tools/list` via stdio.
- **Expected:** Paridade com CLI (mem/verify/review/drift/guard) ou docs claros do subset.
- **Actual:** version/echo/env/run/graph_*; sem mem/verify/review.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift de `rd.md` fora do Scope (até incluirmos no ticket).
- Abort de verify ao tocar `.toolbelt/thresholds.yaml` (path protegido hardcoded).
- Falha imediata do default `go test` neste repo (ruído, mas sinal de misconfig).
- Syntax error em módulo importado pelo caminho de teste configurado (quando `sessions.py` foi corrompido, pytest digest falhou e verify ESCALATE).
- Index/search do grafo (após contornar SkipDirs) localizou `build_digest_header`.

**Deixou passar**

- Ausência de entrada em `HISTORY.md` / aderência a CONTRIBUTING (completude da contribuição).
- Falso positivo de invariante `tests/hidden` no review (bloqueou sem violação real).
- Erro de sintaxe em `tests/test_utils.py` com `test_cmd` só digest → verify PASS.
- Suíte oculta efetivamente desligada sem pasta `tests/hidden/`.
- `guard.yaml` do usuário ignorado (auth.py “protegido” no YAML não gerou violation).
- Impacto de mudança / callers para guiar testes relacionados.

**Nativo vs tb:** ruff e pytest digest validaram o fix; tb verify com o mesmo pytest também PASS. A suite nativa ampla ainda tem 4 timeouts de rede independentes do patch. tb não substituiu lint/typecheck/build Python.

## Interação com instrumentação existente / onboarding

Requests **não** tem harness de agente. Só `AI_POLICY.md` (política social/legal).

`tb init` + `tb mem init` geraram:

- `.toolbelt/config.yaml` (sandbox/graph/vault/logging/bench/verify)
- `.toolbelt/invariants.yaml` (3 regras anti-cheat)
- dirs `bench/`, `logs/`, `vault/`
- depois: `graph.json`, workspaces

**Qualidade:** estrutura ok para um projeto Go; **ruim** como bootstrap Python.

**Faltou**

- Detectar linguagem e comandos reais (`pytest`, `ruff check`, `python -m build`)
- Gerar `AGENTS.md` / skills locais com convenções do repo (e respeitar `AI_POLICY.md`)
- Scaffold de `rd.md` / ticket a partir da issue
- Criar `tests/hidden/` ou documentar que verify oculto não roda sem isso
- Excluir SkipDirs quando o nome coincide com a raiz
- Wire MCP com as mesmas ferramentas do fluxo (verify/mem/review)

## Sugestões de melhoria

1. **[P0 · graph]** Não aplicar `SkipDirs` ao diretório raiz do walk; comparar path relativo ≠ `.`.  
2. **[P0 · init/verify]** Detectar toolchain (Go/Python/JS) e preencher `verify.test_cmd` / sugerir lint+build; nunca defaultar `go test` em repo sem `go.mod`.  
3. **[P1 · verify]** Tratar `--hidden-test-cmd` / config como fonte de verdade; não exigir `tests/hidden/` para marcar disponível.  
4. **[P1 · review]** Eliminar FP de invariantes: casar paths **alterados**, não substrings no diff/ticket; ignorar menções em `rd.md`/`forbidden`.  
5. **[P1 · guard]** Carregar `.toolbelt/guard.yaml` (docs prometem); manter hardcoded só como default mergeável.  
6. **[P1 · docs/cli]** Sincronizar `docs/cli.md` / README com Cobra real (`mem add` posicional; remover ou marcar `tb md`; flags `-o`).  
7. **[P2 · bench]** Honrar `bench.default_agent`; adicionar `--agent` em `bench compare`.  
8. **[P2 · graph]** Melhorar impacto/callers Python (imports/refs a testes); falhar alto se index=0 nós.  
9. **[P2 · onboarding]** Opcional `tb init --agents-md` gerando AGENTS.md mínimo + pointer para verify/mem.  
10. **[P3 · mcp]** Expor mem/verify/review/drift/guard no MCP para fluxo end-to-end no host.  
11. **[P3 · completeness]** Checklist de fechamento (changelog, testes, lint) no review/`acceptance` com falha explícita se critério não coberto por verify.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time do estudo (evidência tb ~20:52Z → ~20:57Z + setup/clone/suite) | ~15 min de uso ativo do tb; suite pytest baseline ~332 s; estudo ponta a ponta ~25–30 min incluindo toolchain |
| Invocações tb (aprox., seções de evidência + runs) | ~32 comandos `tb *` distintos/repetidos |
| Falhas tb (exit ≠ 0 ou Error) | ≥15 (flags docs, impact sem file, drift/review bloqueantes, bench sem agent, verify go default, etc.) |
| Crashes | Nenhum panic/segfault observado |
| Clone alvo | Somente local; zero interação com GitHub do `psf/requests` |
| Artefato neste repo | Apenas este arquivo em `docs/campo/requests.md` na branch `campo/requests` |
