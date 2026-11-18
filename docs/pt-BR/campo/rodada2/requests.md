[🇺🇸 English](../../../campo/rodada2/requests.md) | 🇧🇷 Português

# Campo rodada 2: psf/requests

Grupo: **without existing AI-agent instrumentation**.  
Ferramenta: `tb` compilado de `main` (`make build`, ldflags `a46c295`).  
Clone local: `/tmp/target` (basename `target` de propósito para revalidar SkipDirs).  
Nenhuma interação com o remoto `psf/requests` (sem push/issue/PR/comentário).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [psf/requests](https://github.com/psf/requests) |
| Linguagem | Python ≥3.10 |
| Tamanho | ~34 `.py` em `src/`+`tests/`; ~6396 LOC em `src/requests`; working tree ~5,8 MiB (sem `.git`/`.venv`/`.toolbelt`); shallow clone depth 50 |
| Commit base | `3c7baaa` |
| Instrumentação de IA existente | **Não** há `AGENTS.md`, `CLAUDE.md`, `.cursorrules`. Há `.github/AI_POLICY.md` (política humana). |
| Toolchain | **Parcial.** `python3.12` + venv + `requirements-dev.txt` + editable install + `ruff` + `pytest`. Suite alvo do patch: **13 passed**. **Não configurado:** `tox` multi-versão, `pyright`, docs Sphinx, pre-commit completo. Suite completa não reexecutada (ambiente com timeouts de rede conhecidos da rodada 1). |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Em `should_bypass_proxies`, normalizar entradas IPv6 **entre colchetes** em `no_proxy` (ex.: `[2001:db8::1]`) para casar com `urlparse().hostname` (sem colchetes). Teste parametrizado em `tests/test_utils.py`. |
| Por quê | Bug claro, repro local sem rede, escopo pequeno, distinto da rodada 1. |
| Issue | Nenhuma issue aberta consultada (protocolo: sem rede além de clone/deps). Motivado por falha local: `no_proxy='[2001:db8::1]'` não bypassava `http://[2001:db8::1]/`. |
| Completada? | **Sim no clone local.** Diff: 2 linhas em `utils.py` + 4 casos de teste. Sem `HISTORY.md` / PR upstream (proibido). |
| Checks nativos | `pytest tests/test_utils.py -k 'bypass_proxies_no_proxy_ipv6 or bypass_proxies_no_proxy_domain'`: **13 passed**. `ruff check` nos tocados: **ok**. |
| Rodada 1 (comparação) | Digest `qop` com espaços após vírgula (`auth.py` + `test_lowlevel.py`, issue #7648). |

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|---------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph index --enable .` em `/tmp/target` → **853 nós, 2349 arestas** (rodada 1: 0 nós no mesmo path) |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; config com `hidden_test_cmd` pytest domain → JSON `hidden_available: true`, `hidden_passed: true`, verdict PASS |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff tocando path protegido → JSON com `severity:error` e **EXIT=1** |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `src/requests/adapters.py`; commit probe → `protected_path` nesse arquivo |
| 5 | init detecção de toolchain | **funcionou** | `tb init` → `verify.test_cmd: pytest` (não `go test`); probe `/tmp/agents-probe` idem |
| 6 | review `tests/hidden` casa por path | **funcionou** | Ticket menciona `tests/hidden/**` só como forbidden/prosa; review limpo (`findings: []`, sem FP critical) |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Sem `--agent` com `default_agent: echo…` → `agent=external`, EXIT=0; sem default → EXIT=1; `compare --help` lista `-a/--agent`; A/B com `--agent` rodou |
| 8 | review `## Scope`/`## Escopo`, listas MD e `**` globs | **funcionou** | Ticket só com `## Scope` + bullets → review PASS; frontmatter `src/requests/**/*.py` + `tests/**` cobriu `utils.py`/`test_utils.py` |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | JSON: `build_ran/passed: true`, `lint_ran/passed: true` antes dos testes |
| 10 | init lê AGENTS/CLAUDE e `--write-agents` | **funcionou** | Flag existe; em repo sem docs, `tb init --write-agents` semeou `AGENTS.md` + `.toolbelt/agent_docs.yaml`. Em `/tmp/target` (sem instrumentação) init sem flag não cria AGENTS (esperado) |
| 11 | graph Python `calls` | **funcionou** | Stats: `edges_calls: 1170`. `callers --function should_bypass_proxies` listou testes + `get_environ_proxies`; `impact --file src/requests/utils.py` não vazio |
| 12 | visual skipped agregado + docs CLI alinhados | **funcionou** | Manifesto com refs ausentes → top-level `"skipped": true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` documenta `tb mem add <regra>` (sem `--type` fantasma); CLI rejeita `--type` (EXIT=1) de forma consistente com a docs atual |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; `test_cmd: pytest` | **ajudou** | Onboarding Python correto vs rodada 1 (`go test`) |
| `tb init --write-agents` | Semeia `AGENTS.md` | **ajudou** | Probe `/tmp/agents-probe`; mode 0600 |
| `tb mem init/list/check/add` | OK | neutro/ajudou | Invariantes anti-cheat; `mem add --type` ainda erro (docs ok) |
| `tb graph index` em `/tmp/target` | 853 nós / 2349 arestas | **ajudou** | SkipDirs raiz corrigido |
| `tb graph search/callers/impact` | callers/impact úteis em Python | **ajudou** | Diferente da rodada 1 (vazio) |
| Ticket `rd.md` + `## Scope` | Parse OK | **ajudou** | Heading+bullets e `**` globs |
| `tb verify run` | PASS com build/lint/hidden | **ajudou** | Sem pasta `tests/hidden/` |
| `tb verify --ruler visual` | PASS skipped agregado | neutro | Sem UI real no projeto |
| `tb review` | Limpo com scope correto | **ajudou** | Sem FP `tests/hidden` |
| `tb drift` | Sem out-of-scope no fix | ajudou | Alinhado ao review |
| `tb guard check --strict --json` | EXIT=1 com violação | **ajudou** | `guard.yaml` honrado |
| `tb bench run` / `compare --agent` | OK com fixtures do toolbelt | neutro | Repo alvo sem `bench/tasks` próprios |
| `tb mcp` tools/list | Subset JSON-RPC | neutro | echo/env/run/graph_*/version; sem mem/verify/review |
| `tb version` | `a46c295` | neutro | Com ldflags |

## Problemas que continuam

- **MCP incompleto** frente à CLI (sem mem/verify/review/drift/guard) — igual à rodada 1.
- **Verify só tão amplo quanto `test_cmd`**: com filtro `-k ipv6`, erros em outros arquivos não entram no veredicto (mesmo padrão da rodada 1).
- **Init não preenche `lint_cmd`/`build_cmd`** automaticamente (só `test_cmd`); ainda é edição manual do YAML.
- **`graph impact` exige `--file`** (UX rígida inalterada).
- **Configs em mode `0600`** (confirmado em `config.yaml` / `AGENTS.md` semeado).
- **Repo alvo sem tasks de bench**: precisa apontar `-t/-f` para o tree do toolbelt.
- Docs de arquitetura/`cli.md` ainda mencionam `--manifest`/promessas extras em trechos; o contrato efetivo de `guard check <base>..<head>` exige range posicional (UX fácil de errar).

## Problemas novos

### P2 — Arestas `calls` Python com alvos espúrios

- **Repro:** Após `tb graph index --enable .`, inspecionar edges `calls` (ex.: `file:setup.py` → `func:tests.setup`, confidence 0.55).
- **Esperado:** Resolução conservadora dentro do módulo / imports reais.
- **Actual:** Heurística gera callers/impact úteis, mas também pares de baixa precisão que podem confundir o agente.

### P2 — Onboarding ainda exige YAML manual para lint/build/hidden

- **Repro:** `tb init` em requests → `lint_cmd`/`build_cmd` ausentes; `hidden_test_cmd: ""`.
- **Esperado:** Sugerir `ruff`/`pytest` subset a partir de `pyproject.toml`/`Makefile`.
- **Actual:** Só `test_cmd: pytest`; build/lint/hidden configurados à mão neste estudo.

### P3 — Formato do manifesto visual pouco óbvio

- **Repro:** YAML com chave `reference:` singular não agrega como o schema `references: {light: …}`; com schema certo, skipped agregado funciona.
- **Esperado:** Erro/aviso de schema ou aceitar alias.
- **Actual:** Silêncio / comportamento diferente conforme o shape do YAML.

## O que o tb pegou e o que deixou passar

**Pegou**

- Indexação em `/tmp/target` (regressão SkipDirs sumiu).
- Callers dos testes de `should_bypass_proxies` via arestas `calls`.
- Drift/review com `## Scope` e globs `**`.
- Build+lint+hidden no verify sem pasta `tests/hidden/`.
- Violação `guard.yaml` em `adapters.py` com `--strict --json` (exit 1).
- Seed de `AGENTS.md` via `--write-agents` em repo sem instrumentação.

**Deixou passar**

- Completude de contribuição (`HISTORY.md` / checklist CONTRIBUTING) — nenhum nudge.
- Cobertura além do `test_cmd` estreito.
- Qualidade/precisão de algumas arestas `calls`.
- Política em `.github/AI_POLICY.md` (não importada para mem/invariantes).

**Nativo vs tb:** pytest+ruff validaram o fix; `tb verify` com os mesmos comandos também PASS. tb não substituiu tox/pyright/suite completa.

## Interação com instrumentação existente / onboarding

Requests **não** tem harness de agente; só `AI_POLICY.md`.

`tb init` + `mem init` criaram `.toolbelt/` (config, vault, logs, bench, invariants). Com `--write-agents` (probe), gerou `AGENTS.md` mínimo apontando verify/guard/mem.

**Melhoria vs rodada 1:** defaults Python corretos + opção de seed AGENTS.  
**Ainda falta:** importar dicas de `Makefile`/`pyproject` para lint/build; referenciar `AI_POLICY.md`; template de `rd.md`.

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| SkipDirs `/tmp/target` | 0 nós | 853 nós | **melhor** |
| init `test_cmd` | `go test ./...` | `pytest` | **melhor** |
| graph callers/impact Python | vazio | populado (`calls`) | **melhor** |
| hidden sem dir | ignorado | executa | **melhor** |
| guard.yaml | ignorado | lido | **melhor** |
| `--strict --json` | frouxo (quando testado) | EXIT=1 | **melhor** |
| review FP `tests/hidden` | sim | não | **melhor** |
| `## Scope` + `**` | frágil | OK | **melhor** |
| build/lint no verify | inerte | honrados | **melhor** |
| default_agent / compare `--agent` | quebrado | OK | **melhor** |
| AGENTS / `--write-agents` | ausente | funciona | **melhor** |
| visual skipped agregado | parcial | OK no cenário all-skipped | **melhor** |
| MCP subset / completude tarefa | lacunas | lacunas | **igual** |
| Precisão de `calls` | N/A | ruído novo | **misto** |

No agregado: **claramente melhor** que a rodada 1 para o fluxo end-to-end neste repo; as 12 correções listadas **funcionaram** sob `tb@a46c295`.

## Sugestões de melhoria

1. **[P1 · graph]** Filtrar / ranquear `calls` Python por confiança e escopo de módulo; ocultar pares &lt;0.6 no `callers` default.  
2. **[P1 · init]** Preencher `lint_cmd`/`build_cmd` a partir de `pyproject`/`Makefile` (ex. `ruff check`, `python -m build`).  
3. **[P2 · verify]** Aviso quando `test_cmd` é um subset (`-k` / path único) sem `hidden_test_cmd` amplo.  
4. **[P2 · guard]** Aceitar flags `--base/--head` além de `base..head` (UX alinhada a review/drift).  
5. **[P2 · visual]** Validar schema do manifesto e alias `reference`→`references`.  
6. **[P2 · mcp]** Expor mem/verify/review/guard no MCP.  
7. **[P3 · onboarding]** Ao detectar `.github/AI_POLICY.md`, registrar pointer em `agent_docs.yaml` / mem.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (estudo ativo tb + patch + checks) | ~25–35 min (inclui rebuild após fast-forward de `main`) | ~25–30 min |
| Invocações `tb` (aprox.) | ~45 (revalidação pós-`a46c295` incluída) | ~32 |
| Falhas tb (exit ≠ 0 esperadas/probes) | ~8 (flags docs antigas no binário stale, guard sem range, bench sem tasks, mem `--type`, probes) | ≥15 |
| Crashes | Nenhum | Nenhum |
| Binário final | `toolbelt a46c295` | `c8aae04` |
| Clone alvo | Somente local; zero interação GitHub `psf/requests` | idem |
| Artefato | Apenas `docs/campo/rodada2/requests.md` na branch `campo2/requests` | `docs/campo/requests.md` |
