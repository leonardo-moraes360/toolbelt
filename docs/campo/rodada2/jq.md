# Estudo de campo (rodada 2): jqlang/jq

Grupo: without existing AI-agent instrumentation.  
Ferramenta: `tb` buildado deste repo em `main` (`make build` → `bin/tb`, ldflags `a46c295`).  
Clone alvo: `/tmp/target` (somente local; nenhum push/issue/PR/comentário no upstream).  
Data: 2026-11-16.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [jqlang/jq](https://github.com/jqlang/jq) |
| Linguagem | C (autotools + oniguruma embutido) |
| Tamanho | shallow + submodules ≈ 55–56 MiB no disco; ~27 kLOC em `src/` (`.c`/`.h`); 45 arquivos `.c`/`.h` em `src/` |
| Instrumentação IA | Nenhuma na entrada (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, Copilot, Aider ausentes). Só CI em `.github/workflows/` |
| Toolchain | **Sim, completa**: `autoconf`/`automake`/`libtool`/`flex`/`bison` instalados na sessão; `git submodule update --init`; `autoreconf -i`; `./configure --with-oniguruma=builtin --disable-docs`; `make -j`; `make check` baseline 9/9 PASS. Docs HTML não gerados. Sem Valgrind/scan-build nesta sessão |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Fazer assignment a fatia de array além do comprimento atual paddar com `null` (como `.[10] |= 2`), em `jv_set` / `parse_slice` (`src/jv_aux.c`), com regressões em `tests/jq.test` |
| Por quê | Bug maintainer-filed, escopo pequeno, diferente da rodada 1, verificável com `make check` e one-liners |
| Issue | https://github.com/jqlang/jq/issues/2813 |
| Completa? | **Sim** (localmente): `jq -ncM '[1] \| .[10:11] \|= [2]'` → `[1,null,…,null,2]` (igual a `.[10] \|= 2`); `make check` 9/9 PASS |
| Checks nativos | `make -j` OK; `make check` 9/9 PASS após o fix (~8 s). Sem linter C dedicado no repo além do que CI/scanbuild faria (não rodado) |
| Rodada 1 (comparação) | Rejeitar literais JSON com ponto decimal final (`23.`) em `src/jv_parse.c` / issue #2414 |

Ticket: `rd.md` com `## Scope` / `## Escopo`, listas Markdown, globs `**`, `base_commit` e `forbidden` (menção a `tests/hidden/**`).

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|---------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Clone em `/tmp/target` (basename `target`): `tb graph --enable index .` → **1064 nós / 1530 arestas** (não zera mais) |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; com `hidden_test_cmd: echo HIDDEN_RAN && exit 0` → `hidden_available: true`, `hidden_passed: true`, verdict PASS |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff em `.toolbelt/thresholds.yaml` → JSON com `protected_path` e **exit 1** com `--strict --json` |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `src/jv_aux.c`; `tb guard check BASE..HEAD --strict --json` → violação nesse arquivo, exit 1 |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em jq → `verify.test_cmd: make check` (via `Makefile` / target `check`) |
| 6 | review `tests/hidden` casa por path | **funcionou** | Ticket menciona `forbidden: tests/hidden/**` / invariante anti-cheat; review **não** gerou FP de invariante só por substring — só drift de `rd.md` fora do scope |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Sem `--agent`/`--fake`, com `bench.default_agent` setado → roda (`agent_output` contém o echo do agente). `tb bench compare ... --agent 'echo AGENT {prompt}'` aceito (flag existe e executa A/B) |
| 8 | review/drift `## Scope`/`## Escopo`, listas MD e globs `**` | **funcionou** | Ticket só com `## Scope` + bullets: `src/jv_aux.c`/`tests/jq.test` no escopo; `rd.md` fora. Glob `src/**/*.c` + `tests/**` também casa `src/jv_aux.c` (só `rd.md` out_of_scope) |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Com `build_cmd: make -j2`, `lint_cmd: true` → `build_ran/passed` e `lint_ran/passed` true no PASS. Com `build_cmd: false` → `build_passed: false`, verdict `ESCALATE` |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** | Repo sem instrumentação: `tb init --write-agents` semeou `AGENTS.md` + `.toolbelt/agent_docs.yaml` (`sources: [AGENTS.md]`). Sem a flag, log sugere `--write-agents` |
| 11 | graph Python `calls` edges | **não se aplica** (repo C); fix **confirmado** em probe | Em jq: `edges_calls: 19` (C superficial). Probe `/tmp/py-calls-probe/m.py` (`a` chama `b`): `edges_calls: 1`; `callers --function b` → `func:m.a` |
| 12 | verify visual skipped agregado + docs CLI | **funcionou** | Manifesto com ref ausente: top-level `"skipped": true`, `"skip_reason": "todas as telas foram skipped"`. `docs/cli.md` marca `tb ticket` como **não implementado** |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK (`a46c295` com ldflags) | neutro | Build via Makefile |
| `tb init` / `--write-agents` | OK; `make check`; seed `AGENTS.md` | ajudou | Defaults C/Make corretos; onboarding mínimo gerado |
| `tb mem init/list/add/check` | OK; 3+1 invariantes; check sha256 | ajudou / atrapalhou (docs) | `mem add --type` ainda `unknown flag`; posicional OK |
| `tb graph --enable index/stats/search` | 1064 nós; search achou `jv_set` / `parse_slice` | ajudou | SkipDirs na raiz corrigido; útil p/ localizar símbolo |
| `tb graph callers/deps/impact` | callers/`impact` vazios p/ C útil; deps = includes | atrapalhou (C) | `callers --function jv_array_set` → `(vazio)`; `impact --file src/jv_aux.c` → `(vazio)` |
| Ticket (`rd.md`) | Manual; `tb ticket` ausente | neutro | Docs agora dizem “planejado”; fluxo drift/review consome `rd.md` |
| `tb drift` | Detectou `rd.md` fora do `## Scope` | ajudou | Exit 1 + JSON; ainda dumpa Usage no stderr |
| `tb review` | Drift de `rd.md`; sem FP `tests/hidden` | ajudou | Scope Markdown OK; Usage no stderr permanece |
| `tb verify run --ruler tests` | PASS c/ build/lint/test/hidden | ajudou | ~7–8 s com `make check`; hidden roda sem pasta |
| `tb verify` + `guard.yaml` no arquivo editado | FAIL cedo (`protected_path`) | atrapalhou | Proteger o path da tarefa aborta verify antes dos testes |
| `tb verify --ruler visual` | PASS skipped agregado | neutro | Chrome presente; refs ausentes → skip top-level correto |
| `tb guard check` | Lê `guard.yaml`; `--strict --json` exit 1 | ajudou | Hardcoded + YAML |
| `tb bench run` / `compare` | `default_agent` e `--agent` OK; `--fake` OK | ajudou / parcial | Tasks linkadas do toolbelt; `passed: true` mesmo com `exit_code: 1` do agente se a fixture Go passa |
| `tb mcp` | `initialize` + `tools/list` OK | ajudou | tools grafo + version/echo/env/run |
| Checks nativos vs tb | `make check` alinhado ao verify configurado | — | Verify sem `make check` (ex.: `test_cmd: true`) ainda pode PASS enganoso |

## Problemas que continuam

- **`graph callers` / `impact` pouco úteis em C** (índice superficial; arestas `calls` poucas) — igual rodada 1.
- **`tb ticket` inexistente** (agora documentado como planejado; ainda falta o comando).
- **`mem add` só posicional** (`--type`/`--title` falham).
- **Config mode `0600`** (`config.yaml`, `AGENTS.md` seed, etc.).
- **Usage do Cobra no stderr** após JSON útil em drift/review com exit ≠ 0.
- **Completeness / aceite**: review não valida de fato os checkboxes de acceptance; operador ainda precisa rodar `make check` bem configurado.
- **Verify pode PASS com `test_cmd` fraco** se o humano/config não apontar a suíte real (mitigado em parte por `build_cmd`, mas não eliminado).

## Problemas novos

### P2 — `guard.yaml` no path da própria tarefa aborta `tb verify`

- **Repro**: `protected_paths: [src/jv_aux.c]` + diff que edita esse arquivo; `tb verify run --ruler tests`.
- **Esperado**: guard reporta; verify ainda pode rodar testes **ou** distinguir “path sensível” vs “arquivo do mandate”.
- **Atual**: `verdict: FAIL`, `iterations: 0`, só violação `protected_path` — hidden/build/test nem rodam.

### P2 — `tb bench` marca `passed: true` com agente `exit_code: 1`

- **Repro**: `default_agent: 'printf … && false'`; `tb bench run -r 1 --task bugfix-go`.
- **Esperado**: falha do agente (ou pelo menos `passed: false` se exit ≠ 0).
- **Atual**: `agent_output` mostra o printf, `exit_code: 1`, mas `passed: true` porque a verificação Go da fixture passou.

### P3 — Seed `AGENTS.md` genérico não cita `make check`

- **Repro**: `tb init --write-agents` em jq.
- **Esperado**: dicas alinhadas ao `test_cmd` detectado (`make check`).
- **Atual**: texto harness genérico; `agent_docs.yaml` com `hints: []` (Makefile não vira hint narrativo).

### P3 — `verify` com `build_cmd` falho pode sair exit 0 com verdict `ESCALATE`

- **Repro**: `build_cmd: false`; `tb verify run --json`.
- **Esperado**: exit ≠ 0 quando verdict ≠ PASS.
- **Atual**: JSON `ESCALATE` / `build_passed: false` com exit 0 observado nesta sessão.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift de `rd.md` fora do scope Markdown.
- Guard YAML + hardcoded (`thresholds.yaml`) com exit code estrito.
- Verify com build + lint + `make check` + hidden sem pasta.
- Init com toolchain Make e seed de `AGENTS.md`.
- Graph search dos símbolos tocados; SkipDirs na raiz `/tmp/target` corrigido.
- Visual skipped agregado quando refs faltam.

**Deixou passar / não ajudou**

- Impact/callers não guiaram o patch em C.
- Com `test_cmd: true`, verify PASS sem exercitar a suíte jq (operador ainda manda).
- Bench “sucesso” apesar de agente falhar o exit.
- Review não checou semanticamente os acceptance criteria além de heurísticas/drift.
- Proteger o arquivo da tarefa no `guard.yaml` impede o loop de verify útil.

## Interação com instrumentação existente / onboarding

Repo **sem** instrumentação na entrada (grupo do prompt). Onboarding observado:

| Artefato | Qualidade |
|----------|-----------|
| `.toolbelt/config.yaml` via `tb init` | `test_cmd: make check` adequado; `hidden_test_cmd` veio vazio no init limpo (melhor que `go test` da rodada 1) |
| `AGENTS.md` via `--write-agents` | Criado; genérico toolbelt; mode 0600 |
| `.toolbelt/agent_docs.yaml` | Registra sources; hints vazios |
| `tb mem init` | 3 invariantes anti-cheat genéricas |
| Leitura de `CLAUDE.md` | N/A (ausente); caminho de código exercitado via seed |

Conclusão: bootstrap **melhorou** vs rodada 1 (toolchain + AGENTS), mas o AGENTS semeado ainda não documenta o workflow nativo jq (`autoreconf`/`configure`/`make check`).

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| SkipDirs `/tmp/target` | 0 nós (outros repos) / jq usou subdir | 1064 nós na raiz `target` | **melhor** |
| Init defaults | `go test ./...` | `make check` | **melhor** |
| AGENTS / `--write-agents` | inexistente | seed OK | **melhor** |
| Scope `##` + `**` | frágil / linha `scope:` | headings + `src/**/*.c` OK | **melhor** |
| Review FP `tests/hidden` | confirmado | ausente neste run | **melhor** |
| guard.yaml / `--strict --json` | ignorado / exit 0 | viola + exit 1 | **melhor** |
| hidden sem pasta | não rodava | roda | **melhor** |
| build/lint no verify | ausentes | rodaram | **melhor** |
| visual skipped top-level | mentia | `skipped: true` | **melhor** |
| graph callers/impact em C | vazio | ainda vazio | **igual** |
| Empurrar aceite completo | fraco | ainda fraco | **igual** |
| Tarefa | #2414 parse numérico | #2813 slice assign | diferente (ok) |

No agregado: **melhor** de forma concreta nas 12 correções; gargalos de grafo C e harness de completeness permanecem.

## Sugestões de melhoria

1. **[verify/guard] P1** — Não abortar o loop de testes só porque o path do mandate está em `protected_paths`; ou exigir opt-in / severidade warning para paths declarados no scope.
2. **[bench] P1** — Falhar a task (ou expor `agent_ok`) quando o processo do agente retorna exit ≠ 0, independentemente da fixture.
3. **[verify] P1** — Exit code ≠ 0 para verdicts `FAIL`/`ESCALATE` (mesmo com `--json`).
4. **[init/agents] P2** — Incluir o `verify.test_cmd` detectado no `AGENTS.md` semeado (`make check`, etc.).
5. **[graph] P2** — Melhorar `calls`/impact para C (`#include` + símbolos não-static) — ainda o principal buraco neste repo.
6. **[cli] P3** — Em `--json`, silenciar Usage do Cobra em erros de domínio (drift/review).
7. **[docs/mem] P3** — Alinhar `mem add` docs↔CLI ou aceitar flags nomeadas.

## Métricas

| Métrica | Rodada 2 | Rodada 1 (jq) |
|---------|----------|---------------|
| Wall time sessão (clone+toolchain+fix+tb) | ≈ 20–25 min | ≈ 25–30 min |
| Wall time só invocações `tb` cronometradas (1ª leva) | ≈ 7,6 s somados; verify `make check` ≈ 7,3 s | ≈ 3,75 min span |
| Invocações `tb` registradas | ~30+ (21 na 1ª leva @8b5a499 + reteste @a46c295 + mcp/bench) | 45 |
| Falhas `tb` (exit ≠ 0) | várias esperadas (drift, guard strict, init `--write-agents` no binário antigo, flags inválidas) | 18 |
| `make check` baseline / pós-fix | 9/9 PASS / 9/9 PASS | 9/9 / 9/9 (#2414) |
| Binário final de evidência das 12 correções | `toolbelt@a46c295` | `toolbelt@c8aae04` |

Artefatos locais (não commitados no toolbelt nem no jq upstream): `/tmp/campo-jq-r2/evidence/`, clone `/tmp/target` com commit local `6dafa26` (fix #2813) — **não** enviado ao GitHub do jq.
