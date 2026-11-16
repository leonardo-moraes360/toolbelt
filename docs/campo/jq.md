# Estudo de campo: jqlang/jq

Grupo: sem instrumentação de agente de IA.  
Ferramenta: `tb` buildado deste repo (`make build` → `bin/tb`, versão com ldflags `c8aae04`).  
Clone alvo: `/tmp/target/jq` (somente local; nenhum push/issue/PR/comentário no upstream).  
Data: 2026-11-15.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [jqlang/jq](https://github.com/jqlang/jq) |
| Linguagem | C (autotools + oniguruma embutido) |
| Tamanho | clone shallow + submodules ≈ 19–56 MiB no disco; ~27 kLOC em `src/` (`.c`/`.h`); 50 arquivos `.c`/`.h` fora de `vendor/`, 176 com vendor |
| Instrumentação IA | Nenhuma encontrada (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot, Aider, Gemini). Só workflows CI em `.github/workflows/` |
| Toolchain | **Sim, completa**: `autoconf`/`automake`/`libtool`/`flex`/`bison` instalados; `autoreconf -i`; `./configure --with-oniguruma=builtin --disable-docs`; `make -j`; `make check` (9/9 PASS no baseline, ~12 s). Docs HTML não gerados (`--disable-docs`). Sem Valgrind/scan-build nesta sessão |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Rejeitar literais JSON com ponto decimal final (`23.`, `-23.`, `0.`) em `check_literal` (`src/jv_parse.c`), com regressão em `tests/jq.test` via `fromjson` |
| Por quê | Bug claro, escopo pequeno, alinhado a RFC 8259 (`frac = decimal-point 1*DIGIT`), plausível para maintainer |
| Issue | https://github.com/jqlang/jq/issues/2414 |
| Completa? | **Sim** (localmente). Tentativa anterior da #3128 (patch do maintainer) **falhou** (assertion `jq->subexp_nest > 0`, 5 suítes FAIL) e foi revertida — evidência de que o tb não empurrou para completar/corrigir a primeira tarefa |
| Checks nativos | Após o fix #2414: `make check` → 9/9 PASS (~7,9 s). `printf '23.' \| ./jq .` → exit 5, `Invalid numeric literal`; `23.0` continua OK. Não há linter C dedicado no repo além do que o CI/scanbuild faria (não rodado aqui) |

Ticket usado (manual, `tb ticket` não existe): `rd.md` com frontmatter `scope` / `base_commit` / `acceptance`.

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK (`c8aae04` com ldflags); `go build` sem ldflags → `0.1.0-dev` / `commit: unknown` | neutro | `/tmp/tb-nolf version` mostrou `unknown`; binário do Makefile não |
| `tb init` | Criou `.toolbelt/` + `config.yaml` (0600), dirs `vault`/`logs`/`bench` | ajudou | exit 0 em 5 ms; defaults `verify.test_cmd: go test ./...` inadequados para C |
| `tb mem init/list/add/check` | init gerou 3 invariantes; add posicional OK; flags `--type`/`--title` falham | ajudou / atrapalhou (docs) | `mem add --type fact …` → `unknown flag: --type`; `mem add "…"` OK; `mem check` sha256 + `compactacao_segura: true` |
| `tb graph --enable index/rebuild/stats/search` | Indexou 1064 nós / 1530 arestas em ~0,1 s; search achou `gen_destructure` | ajudou (parcial) | C indexado via `cpp`; `bind_alternation` (static) não apareceu na search |
| `tb graph callers/deps/impact` | callers exige `--function`; deps `--file`; impact exige `--file` e retornou vazio/`null` para `src/compile.c` | atrapalhou | UX confusa (args posicionais rejeitados); impact sem valor prático no C deste repo |
| Ticket + Scope (`rd.md`) | Manual; `tb ticket` inexistente | atrapalhou | `tb ticket` → `unknown command`; docs/cli.md ainda documentam o comando |
| `tb verify run --ruler tests` | Com `--test-cmd 'make check'` → PASS (~8 s); default Go seria inútil | ajudou (se configurado) | JSON: `visible_passed: true`, `hidden_available: false`. Com `--test-cmd './jq -n 1'` também PASS sem exercitar a suíte — verify não força build |
| `tb verify run --ruler visual` | PASS com telas `skipped: true` (ref ausente), mas relatório `"skipped": false` | atrapalhou | Chrome presente; manifesto de exemplo; top-level `skipped` mente |
| `tb review` | Exit 1; FP de invariante `tests/hidden` porque `rd.md` menciona `tests/hidden/**` no diff | atrapalhou | Finding critical mesmo sem tocar `tests/hidden/`; também flagrou `rd.md` fora do scope (correto) |
| `tb drift` | Detectou `rd.md` fora do scope; globs `src/*.c` e `tests/**` **funcionam** | ajudou | Literal e glob testados; `src/*.c` excluiu corretamente `tests/jq.test` quando só C estava no scope |
| `tb guard check` | Sem violações no fix; viola `protected_path` em `.toolbelt/thresholds.yaml` (hardcoded). `guard.yaml` custom ignorado. Exit 0 mesmo com violação; `--strict --json` também exit 0 (early return JSON) | atrapalhou | `.toolbelt/guard.yaml` com `src/jv_parse.c` protegido → ainda `violations: null` no diff do parse |
| `tb bench run --fake` | OK se apontar tasks do toolbelt; no jq falha sem `bench/tasks` | neutro | 9 tasks, 33,3% sucesso, ~4 s; `--fake` **não** exigiu `--fake-config` |
| `tb bench compare` | Sem flag `--agent`; A/B só via `--fake` ou JSONL | atrapalhou | `tb bench compare --agent x` → `unknown flag: --agent` |
| `tb mcp` | Sobe; `initialize` + `tools/list` via JSON-RPC newline | ajudou | tools: `tb_graph_*`, `tb_version`, `tb_echo`, `tb_env`, … |
| Checks nativos vs tb | `make check` pegou a regressão da #3128 (crash); verify configurado só com `./jq -n 1` deixaria passar | — | Ver seção “O que o tb pegou…” |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` falta `--agent` | **confirmado** | help sem `--agent`; flag → `unknown flag: --agent` |
| `guard.yaml` ignorado | **confirmado** | arquivo com `protected: [src/jv_parse.c]`; `guard check` no diff desse arquivo → `violations: null` (só hardcoded) |
| Docs CLI desatualizados | **confirmado** | `docs/cli.md`: `tb ticket`, `mem add --type/--title`, `guard --manifest`; CLI real diverge |
| `version` mostra `unknown` | **confirmado** (sem ldflags) | `go build -o /tmp/tb-nolf` → `commit: unknown` / `built: unknown` |
| Config files 0600 | **confirmado** | `stat`: `config.yaml`, `invariants.yaml`, `graph.json` → mode `600` |
| Visual ruler `skipped=false` ao pular | **confirmado** | verify visual: screens `skipped: true`, top-level `"skipped": false`, `verdict: PASS` |
| `default_agent` ignorado | **confirmado** | `bench.default_agent: echo-should-be-used` no config; `tb bench run -r 1` → `--agent ou --fake é obrigatório` |
| Guard aborta verify ao tocar path hardcoded de config | **confirmado** | commit em `.toolbelt/thresholds.yaml` → verify `FAIL`, `iterations: 0`, `protected_path` |
| Default fake agent precisa `--fake-config` | **refutado** | `tb bench run --fake` sem `--fake-config` rodou (usa `FakeAgentCommand("")`) |
| `mem add` posicional | **confirmado** | CLI: `tb mem add <regra>`; docs com flags nomeadas falham |
| Drift/Scope precisam paths literais (sem globs) | **refutado** | `src/*.c` e `tests/**` casam via `inScope` (`filepath.Match` / prefixo `/**`) |
| `graph impact` precisa `--file` | **confirmado** | sem flag → `--file é obrigatório`; com flag → saída vazia/`null` neste C |
| Review FP na invariante `tests/hidden` | **confirmado** | `suggestsViolation` acha substring `tests/hidden` no diff do próprio `rd.md` (`forbidden:`) |
| Verify roda checks parciais e perde erros de build | **confirmado** | `--test-cmd './jq -n 1'` → PASS sem `make`; default config é `go test` |
| tb não empurra a completar a tarefa inteira | **confirmado** | 1ª tarefa (#3128) abandonada após patch quebrado; tb review/verify não sinalizaram “aceite incompleto” além do que o operador configurou; só `make check` nativo mostrou o crash |

## Problemas novos

### P1 — `tb guard check --strict --json` não falha o exit code

- **Repro**: `tb guard check 189e148..HEAD --strict --json` com violação `protected_path`.
- **Esperado**: exit ≠ 0 quando há `SeverityError` e `--strict`.
- **Atual**: imprime JSON e retorna 0 porque o ramo `--json` retorna antes do bloco `strict`.

### P2 — `graph impact` / `callers` inúteis ou vazios em projeto C indexado

- **Repro**: `tb graph --enable index` (1064 nós); `tb graph --enable impact --file src/compile.c` → `(vazio)` / JSON `null`; `callers --function gen_destructure` → vazio.
- **Esperado**: impacto/callers a partir dos nós `function` indexados.
- **Atual**: indexação superficial; arestas `calls` poucas (19); impact não ajuda o workflow.

### P2 — Onboarding não detecta toolchain do repo

- **Repro**: `tb init` em jq.
- **Esperado**: sugerir/preencher `make check` / `make` / ausência de linter Go.
- **Atual**: `verify.test_cmd: go test ./...`, `hidden_test_cmd: go test ./tests/hidden/...`; zero `AGENTS.md` ou skills.

### P3 — `tb review` trata menção a path proibido no ticket como violação

- **Repro**: `forbidden: [tests/hidden/**]` em `rd.md` commitado; `tb review` no mesmo range.
- **Esperado**: só flagrar alteração real sob `tests/hidden/`.
- **Atual**: critical “possível violação” por substring no diff.

### P3 — Saída de erro do Cobra dumpa Usage após JSON útil

- **Repro**: qualquer `tb drift`/`review` com drift (exit 1).
- **Esperado**: JSON + exit 1, sem Usage longo no stderr.
- **Atual**: Usage atrapalha agentes que parseiam stderr.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift de `rd.md` fora do `scope` declarado.
- Guard/verify bloqueando alteração de `.toolbelt/thresholds.yaml`.
- Verify com `make check` alinhado ao nativo (PASS após #2414).
- Mem check de integridade das invariantes.
- Graph search localizou símbolos C relevantes (`gen_destructure`).

**Deixou passar / não ajudou**

- Patch quebrado da #3128: verify só falharia se `test_cmd` fosse a suíte real; com comando parcial, PASS enganoso.
- Review não validou acceptance criteria de forma confiável (heurística de tokens); FP em invariante.
- Impact/callers não guiaram o patch.
- Nenhuma pressão para rodar `make`/`make check` além do que o humano configurou.
- Build errors não entram no verify a menos que estejam no `test_cmd`.

## Interação com instrumentação existente / onboarding

Repo **sem** instrumentação de agente. O onboarding do tb a partir do zero:

| Gerado por `tb init` / `mem init` | Qualidade |
|-----------------------------------|-----------|
| `.toolbelt/config.yaml` | Genérico Go; errado para jq |
| `.toolbelt/vault`, `logs`, `bench` | Vazios / placeholders |
| `.toolbelt/invariants.yaml` (via `mem init`) | 3 regras anti-cheat úteis em abstrato, mas centradas em `tests/hidden` e thresholds (ecossistema Go/tb) |
| `AGENTS.md` / detecção de `Makefile`/`configure` | **Não gerado / não detectado** |
| Comandos test/lint/build | Não inferidos; operador teve de setar `make check` à mão |

Conclusão: bootstrap é “esqueleto toolbelt”, não “projeto compreendido”. Para C/autotools, o agente ainda precisa descobrir a toolchain sozinho.

## Sugestões de melhoria

1. **[verify/init] P0** — Detectar `Makefile`/`configure.ac`/`Cargo.toml`/etc. no `tb init` e preencher `verify.test_cmd` / documentar build (ex.: `make check`).
2. **[init/mem] P0** — Opção de gerar `AGENTS.md` mínimo (build, test, layout) quando não existir instrumentação.
3. **[docs] P1** — Sincronizar `docs/cli.md` com a CLI real (remover `ticket` ou implementar; corrigir `mem add`; `guard` sem `--manifest` até existir).
4. **[guard] P1** — Honrar `.toolbelt/guard.yaml`; corrigir `--strict` com `--json` (checar severidade antes/depois do encode).
5. **[verify/visual] P1** — Se todas as telas forem skip, `skipped: true` no relatório e/ou verdict distinto de PASS “cheio”.
6. **[review] P1** — Matching de invariantes por path alterado real, não substring no diff/ticket.
7. **[bench] P1** — Respeitar `bench.default_agent`; aceitar `--agent` em `compare` ou documentar só `--fake`/JSONL.
8. **[graph] P2** — Impact/callers com fallback útil quando o índice C não tem arestas `calls` (ex.: co-ocorrência no mesmo TU, `#include`).
9. **[verify] P2** — Modo “repo checks” que rode build+test declarados e falhe se `test_cmd` for o default Go em repo não-Go.
10. **[cli] P3** — Em `--json`, não imprimir Usage do Cobra no stderr em erros de domínio (drift/review).

## Métricas

| Métrica | Valor |
|---------|-------|
| Wall time (comandos tb cronometrados, ~20:46–20:50 UTC) | ≈ 3,75 min de span de invocações; sessão completa (clone+toolchain+implementação+tb) ≈ 25–30 min |
| Invocações tb registradas | 45 linhas em `/tmp/campo-jq-evidence/invocations.tsv` (mais `make`/`git` nativos) |
| Falhas tb (exit ≠ 0) | 18 (inclui casos esperados: drift, review, flags inválidas, graph off, ticket ausente) |
| `make check` baseline | PASS 9/9, ~12,4 s |
| `make check` pós-#2414 | PASS 9/9, ~7,9 s |
| `make check` pós-patch #3128 (abortado) | FAIL 5/9 (assertion), ~1,5 s |
| Bench fake (tasks do toolbelt) | 9 tasks × 1, 33,3% sucesso, ~4,1 s |

Artefatos locais de evidência (não commitados): `/tmp/campo-jq-evidence/`, clone `/tmp/target/jq` com commits locais `189e148` (fix #2414) e `3d4c60d` (probe thresholds) — **não** enviados ao GitHub do jq.
