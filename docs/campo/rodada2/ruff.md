# Estudo de campo rodada 2: `tb` em astral-sh/ruff

Data: 2026-11-16. Clone local em `/tmp/target/ruff` (shallow). Nenhuma interação com o remoto do alvo. Alterações de código ficaram só no clone local; neste repositório toolbelt só este arquivo.

`tb` usado: build local `make build` → `./bin/tb` (versão embutida `a46c295`, main com as 12 correções).

Grupo: **with existing AI-agent instrumentation**.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [astral-sh/ruff](https://github.com/astral-sh/ruff) |
| Linguagem | Rust (monorepo Ruff + ty); Python em fixtures/scripts/resources |
| Tamanho | Checkout shallow ~145–187 MiB (sem `target` completo no uso); ~2057 arquivos `.rs` |
| Instrumentação AI existente | `AGENTS.md` (~20 KiB), `CLAUDE.md` (`@AGENTS.md`), `.agents/` (skills + threat models), hooks/settings Claude |
| Toolchain | **Parcial.** Rust 1.99.0 (`rust-toolchain.toml`), `cargo`, `cargo-nextest` 0.9.152, `uv` 0.13.8. Compilou/testou `ruff_linter` (pep8_naming) e `cargo run -p ruff -- check` no probe. **Não** rodou: suíte workspace completa, `cargo clippy --workspace`, hooks `prek`, playground, file-watcher |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | `N801` não deve flagrar nomes CapWords com underscore **trailing** (`class Fine_`, `class _Fine_`), alinhado ao skip de leading underscores e ao pep8-naming/PEP 8. Continua flagrando underscore no meio (`Bad_Class_`). Fixture + snapshot atualizados. |
| Por quê | Bug real pequeno (#30230), distinto da rodada 1, com harness de snapshots já existente. |
| Issue | https://github.com/astral-sh/ruff/issues/30230 |
| Completa? | **Sim no clone local** (`trim_matches('_')` em `invalid_class_name.rs`; testes pep8_naming 49 ok; `ruff check --select N801` no probe: só `Bad_Class_` e `bad`). Não enviada ao upstream. |
| Checks nativos | `cargo test -p ruff_linter --lib rules::pep8_naming::tests` PASS (~84 s frio / ~0,04 s quente). `cargo check -p ruff_linter` PASS. Clippy/nextest workspace **não** concluídos. |
| Rodada 1 (comparação) | FURB101/103: não emitir fix “safe” para `open(b"...")` → `Path(...).read_*/write_*` |

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Clone útil em `/tmp/target/ruff` → 58926 nós / 96890 arestas. Probe dedicado `/tmp/skipdirs-probe/target` (basename=`target`) → 6+ nós após rebuild (não zera mais). |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; com `hidden_test_cmd: cargo check -p ruff_linter` → `hidden_available: true`, `hidden_passed: true` no JSON do verify. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com violação: `--json` só → EXIT 0; `--strict --json` → EXIT 1 (mesmo payload JSON com `violations`). |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | Path custom `custom_protected_marker.txt` em `.toolbelt/guard.yaml` → violação `protected_path` no commit que o tocava; união com defaults documentada em `docs/cli.md`. |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em ruff → `verify.test_cmd: cargo test` (não mais `go test ./...`). |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket menciona `tests/hidden/` no texto; `tb review --ticket rd.md --json` → `findings: []` (sem FP critical). |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `bench.default_agent: "echo-agent {prompt}"`; `tb bench run -r 1` sem `--agent` → `agent=external` e concluiu. `tb bench compare --help` lista `-a/--agent`; A/B com `--config-a/b --agent "echo-agent {prompt}" --task bugfix-go` gerou tabela Markdown. |
| 8 | review parseia `## Scope`/`## Escopo`, listas MD e `**` globs | **funcionou** | Ticket com `## Escopo` + bullets + `pep8_naming/**` / `snapshots/**`: drift/review sem `out_of_scope` no fix; commit de ruído `lib_noise_campo.txt` → `out_of_scope` detectado. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Config com `build_cmd: cargo check -p ruff_linter`, `lint_cmd: true` → JSON `build_ran/passed` e `lint_ran/passed` true; verify PASS ~19,5 s. |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** (parcial UX) | Em ruff: log `documentação de agentes encontrada` + `.toolbelt/agent_docs.yaml` com hints `cargo`/`uv`/`nextest`. `--write-agents` em repo vazio semeou só `AGENTS.md` (não criou `CLAUDE.md`); hints **não** viram `test_cmd` automaticamente além da detecção Cargo. |
| 11 | graph arestas `calls` em Python | **funcionou** | Stats: `edges_calls: 4526` (rodada 1: 232). Heurística no `graph.json`: ~4294 calls ligadas a `.py` / fixtures; probe `mod.py` → 1 call e `tb graph callers --function helper` retornou `mod.main`. |
| 12 | verify visual skipped agregado + docs CLI alinhados | **funcionou** | Sem manifesto: top-level `skipped: true`. Com manifesto e ref ausente: `skipped: true`, `skip_reason: todas as telas foram skipped`. `docs/cli.md` documenta build/lint, visual skipped, `protected_paths`, `default_agent`, e marca `tb ticket` como não implementado. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; `.toolbelt/` + `config.yaml` + `agent_docs.yaml` | **ajudou** | Detectou Cargo; leu AGENTS/CLAUDE; ainda mode `600` nos YAMLs |
| `tb init --write-agents` | OK em ruff (já existia docs); em probe vazio semeou AGENTS.md | misto | Não semeia CLAUDE.md; não importa skills `.agents/` |
| `tb mem init/list/add/check` | OK; 3 default + 1 regra insta | ajudou | `compactacao_segura: true` |
| `tb graph rebuild --enable` | 58926 nós / 96890 arestas / 4526 calls ~4,57 s | ajudou | Indexação rápida; SkipDirs raiz OK |
| `tb graph search --enable invalid_class_name` | Achou função/arquivo/class | ajudou | ~0,38 s |
| `tb graph impact --enable --file …/invalid_class_name.rs` | `null` | atrapalhou | Call graph Rust ainda fraco (~232 calls “other”) |
| `tb graph callers --enable --function invalid_class_name` | `null` | atrapalhou | Idem |
| `tb graph deps --enable --file …` | pkgs `crate::…` / `ruff_python_ast` | neutro | Imports superficiais |
| ticket (`rd.md` manual) | `tb ticket` inexistente (docs dizem planejado) | neutro | Scope via `## Escopo` funcionou com drift/review |
| `tb drift --ticket rd.md` | Limpo no fix; flagrou ruído fora do glob | **ajudou** | Globs `**` + heading Markdown OK |
| `tb verify run` | PASS com build+lint+test+hidden | **ajudou** | Defaults Cargo; hidden sem dir; ~19,5 s |
| `tb verify run --ruler visual` | PASS skipped (manifesto/ref) | neutro | Agregação top-level correta |
| `tb review --ticket rd.md` | `findings: []`, drift limpo | **ajudou** | Sem FP `tests/hidden` |
| `tb guard check` + `guard.yaml` | Lê YAML; `--strict --json` EXIT 1 | **ajudou** | Custom path e path do fix detectados |
| `tb bench run` (default_agent) | Rodou sem `--agent`/`--fake` | ajudou | 9 tasks / 11,1% sucesso com echo-agent |
| `tb bench compare --agent` | A/B graph-off vs graph-on OK | ajudou | Flag `--agent` presente e funcional |
| `tb mcp` | `tools/list` + `tb_version` + `tb_graph_search` OK | ajudou | Com `TB_GRAPH=on`, search achou `invalid_class_name` |
| Checks nativos (fora tb) | pep8_naming + ruff check N801 OK | ajudou | Confirmou comportamento Fine_/Bad_Class_ |

## Problemas que continuam

Problemas da rodada 1 (`docs/campo/ruff.md`) que **ainda** aparecem neste run:

| problema rodada 1 | status rodada 2 |
|-------------------|-----------------|
| Call graph / impact inúteis em monorepo **Rust** | **continua** — `impact`/`callers` em `invalid_class_name` → `null`; ~232 calls não-Python |
| Config files mode `0600` | **continua** — `stat` → `600` em `config.yaml` / `invariants.yaml` |
| `tb ticket` / board ausentes (só rd.md manual) | **continua** — `unknown command "ticket"`; docs agora marcam planejado |
| Completeness/review não valida aceite comportamental de verdade | **continua** (parcial) — review passou sem ligar a `verify`/teste N801 |
| Init não importa skills `.agents/` / política de snapshots como invariantes | **continua** — só hints em `agent_docs.yaml`; `mem` ainda genérico |
| Defaults de verify ainda genéricos demais sem refinar crate/pacote | **parcialmente mitigado** — `cargo test` em vez de `go test`, mas não o filtro `-p ruff_linter …` do AGENTS.md |
| Guard UX `base..head` vs `--base` | **não revalidado a fundo** — fluxo `HEAD~1..HEAD` usado com sucesso |

## Problemas novos

1. **P2 — `--write-agents` semeia só `AGENTS.md`, não `CLAUDE.md`**  
   - Repro: `tb init --write-agents` em repo vazio → cria `AGENTS.md`; `CLAUDE.md` ausente.  
   - Esperado (pelo enunciado da correção 10): seed de AGENTS/CLAUDE.  
   - Atual: `SeedAgentsMD` só escreve AGENTS.md.

2. **P2 — Hints de AGENTS.md não viram `verify.*_cmd` estruturados**  
   - Repro: init em ruff extrai `cargo nextest …`, `uv run … prek`, etc., mas `test_cmd` fica só `cargo test`.  
   - Esperado: mapear lint/test/build sugeridos do AGENTS.md.  
   - Atual: registro informativo em `agent_docs.yaml` sem aplicar ao verify.

3. **P3 — `lint_cmd: true` / agentes echo mascaram qualidade**  
   - Repro: verify PASS com `lint_cmd: true`; bench default_agent echo “passa” tasks sem editar código (11,1%).  
   - Esperado: falhar ou avisar quando lint/test são no-ops.  
   - Atual: harness aceita e reporta PASS/sucesso baixo sem alerta de configuração fraca.

4. **P3 — Skills `.agents/skills/*` ainda fora do tb**  
   - Repro: diretório com skills ty/review presente; nenhum comando tb lista/importa.  
   - Esperado (grupo “com instrumentação”): pelo menos descoberta.  
   - Atual: paralela à instrumentação humana.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift fora do `## Escopo` / globs `**`.
- Review sem FP de `tests/hidden` apesar da menção no ticket.
- Guard com `protected_paths` custom + exit strict+json.
- Verify com build + hidden configurado sem pasta `tests/hidden/`.
- Graph search do símbolo N801; calls Python no índice; SkipDirs em dir chamado `target`.
- Init Cargo + leitura de AGENTS/CLAUDE.

**Deixou passar / atrapalhou**
- Impact/callers Rust não ajudaram a achar quem chama `invalid_class_name`.
- Não sugeriu o comando exato do AGENTS.md (`nextest -p …`, política INSTA_*).
- Não empurrou clippy/workspace; agente teve que escolher o subset.
- Bench com echo-agent não valida a tarefa N801 (só o harness fake/external do toolbelt).
- Sem `tb ticket`, o fluxo de aceite continua manual.

## Interação com instrumentação existente / onboarding

- **Melhoria vs rodada 1:** `tb init` **lê** `AGENTS.md`/`CLAUDE.md`, grava `agent_docs.yaml` e lista hints de comandos; `--write-agents` documentado e funcional para seed mínimo.
- **Ainda paralelo:** invariantes default (`tests/hidden/`, `thresholds.yaml`) não refletem o harness Ruff (insta/mdtest/nextest). Skills em `.agents/skills/` e threat models **não** entram no vault/`mem`.
- **Conflito residual:** `test_cmd: cargo test` é melhor que Go, mas o AGENTS.md manda `cargo nextest` + vars `INSTA_*` / exclusão `file_watching` — o agente ainda precisa ler o AGENTS.md por fora para o workflow correto.
- Onboarding: segundos para init/mem; onboarding *real* ao Ruff ainda depende do humano/AI abrir AGENTS.md.

## Comparação com a rodada 1

| aspecto | rodada 1 (`c8aae04`) | rodada 2 (`a46c295`) |
|---------|----------------------|----------------------|
| Init / verify defaults | `go test ./...` | `cargo test` + build/lint/hidden opcionais |
| AGENTS.md | ignorado | lido + hints; skills ainda fora |
| Review FP `tests/hidden` | critical FP | ausente |
| Scope Markdown / `**` | frágil / parcial | `## Escopo` + globs OK neste run |
| guard.yaml / strict+json | ignorado / exit 0 | honrado / EXIT 1 |
| Graph em `/tmp/target…` | risco SkipDirs | raiz OK; calls Python ↑; Rust impact igual de fraco |
| Bench default_agent / compare --agent | quebrado | funciona |
| Visual skipped top-level | falhava com refs ausentes | `skipped: true` agregado |
| Conclusão | tb atrapalhava bootstrap Rust | **melhor** no harness; **igual** no call-graph Rust e na profundidade de onboarding das skills |

No geral: **melhor** de forma concreta nas 12 correções testáveis; a dor estrutural do monorepo Rust (impact/callers) e o aproveitamento fino da instrumentação AI **permanecem**.

## Sugestões de melhoria

1. **[graph/P1]** Emitir `calls`/`impact` úteis para Rust (ou documentar limitação e degradar UX de impact vazio).  
2. **[init/P1]** Aplicar hints do AGENTS.md a `verify.test_cmd` / `lint_cmd` / `build_cmd` (ex.: preferir `cargo nextest` quando citado).  
3. **[init/P2]** `--write-agents` também semear ou linkar `CLAUDE.md`; opcional `--import-skills` de `.agents/skills`.  
4. **[mem/P2]** Importar proibições do AGENTS.md (“não editar snapshots à mão”) como invariantes.  
5. **[verify/P2]** Avisar quando `lint_cmd`/`test_cmd` forem no-ops (`true`, `echo`).  
6. **[bench/P3]** No alvo sem `bench/tasks`, falhar com mensagem clara em vez de depender de path absoluto do toolbelt.  
7. **[cli/P3]** Manter docs de `ticket`/`board` claramente “planejado” (já melhorou) e apontar template `rd.md` com `## Escopo`.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time (aprox., após build tb) | ~15–25 min tb+cargo no alvo (incl. ~84 s teste frio pep8_naming; graph ~4,6 s; verify ~19,5 s; build ruff binário para probe N801 ~1–2 min) | ~6–15 min; graph 4,96 s; cargo test frio refurb ~87 s |
| Invocações tb (marcas `===` / famílias) | ~25 seções no log `/tmp/tb-campo-ruff-r2.log` + probes extras (guard/bench/MCP/write-agents) | ~25 famílias / ~36 seções |
| Falhas / erros tb observados | `graph impact`/`callers` vazios em Rust; `tb ticket` unknown; drift EXIT≠0 no probe de ruído (esperado); guard EXIT 1 com `--strict` (esperado) | similares + vários bugs agora corrigidos |
| Crashes tb | Nenhum | Nenhum |
| MCP | `tools/list` + `tb_version` + `tb_graph_search` OK | OK |

Log bruto local (VM): `/tmp/tb-campo-ruff-r2.log`. Clone e commits de estudo **não** foram enviados a `astral-sh/ruff`.
