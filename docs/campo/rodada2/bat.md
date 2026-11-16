# Estudo de campo rodada 2: sharkdp/bat

Grupo: **without existing AI-agent instrumentation**. Clone local em `/tmp/target/bat`. Nenhuma interação com o upstream. `tb` buildado deste repo em `main` (`make build` → `toolbelt a46c295`, com as 12 correções mergeadas).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [sharkdp/bat](https://github.com/sharkdp/bat) |
| Linguagem | Rust (edition 2021, MSRV `1.88`) |
| Tamanho | shallow clone ~17 MiB código; ~67 arquivos `.rs` / ~19 632 LOC; `target/` local após build |
| HEAD estudado | `fbd9a41` (upstream) + commit local `6a2710c` (somente no VM) |
| Instrumentação AI existente | **Nenhuma** antes do estudo (`AGENTS.md`/`CLAUDE.md`/`.cursorrules` ausentes). `tb init --write-agents` semeou `AGENTS.md` mínimo + `.toolbelt/agent_docs.yaml` |
| Toolchain | Rustup `stable` 1.100.0 (upgrade a partir de 1.83.0 — insuficiente para MSRV); `cargo build --bins`, `cargo test --test integration_tests …`, `cargo fmt`, `cargo clippy --bins` OK |
| Não configurado | Submódulos `assets/syntaxes/*` / temas não inicializados (assets pré-compilados bastaram). Suite integration completa (~290 testes) e syntax-tests/snapshots não rodadas por tempo |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Evitar panic `capacity overflow` em `--line-range` com offset-from-end enorme (`:-N` / `-N:` com N ≈ `usize::MAX`) |
| Por quê | Bug real aberto, escopo pequeno-médio, caminho claro de teste de integração; **diferente da rodada 1** |
| Issue | https://github.com/sharkdp/bat/issues/3845 (também #4039) |
| Completada? | **Sim** (localmente): `src/controller.rs` (`saturating_add` + `VecDeque::new`/`try_reserve` em vez de `with_capacity`), 2 testes em `tests/integration_tests.rs`, entrada no `CHANGELOG.md` |
| Checks nativos | `cargo build --bins` OK; `cargo fmt -- --check` OK; `cargo clippy --bins -- -D warnings` OK; testes `line_range_huge_offset_from_end_does_not_panic` e `line_range_huge_offset_from_start_of_end_does_not_panic` OK |
| Rodada 1 (comparação) | Validar `--language`/`--map-syntax` com color never (issue #4120) — tarefa distinta |

Causa raiz: `print_file_ranges` fazia `largest_offset_from_end() + 1` e `VecDeque::with_capacity(buffer_size)`, o que aborta com `capacity overflow` para offsets enormes.

## Verificação das 12 correções

| # | correção | resultado | evidência |
|---|----------|-----------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | Indexar dir literalmente nomeado `/tmp/campo-bat/target` → `3 nós, 2 arestas` (não 0). Em `/tmp/target/bat` o rebuild indexou `2981` nós |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | `ls tests/hidden` ausente; `tb verify run` com `hidden_test_cmd: cargo test …` → `hidden_available: true`, `hidden_passed: true`, verdict PASS |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Com violações em `guard.yaml`: `--strict --json` → exit **1** + JSON com `protected_path` |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | YAML com `CHANGELOG.md` + `src/controller.rs` → violations nesses paths no diff do fix |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em bat → `verify.test_cmd: cargo test` (não mais `go test ./...`) |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket mencionando `tests/hidden/` só em prosa, sem tocar o path → findings sem FP da invariante; só drift de scope |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Sem flags: usa `echo AGENT_OK {prompt}` (`agent: external`, 100 % em mcp-smoke). Sem default → erro pedindo `--agent`/`--fake`/`default_agent`. `compare --agent` aceito e rodou A/B |
| 8 | review `## Scope`/`## Escopo`, listas MD e globs `**` | **funcionou** | Ticket com `## Escopo` + bullets + `` `tests/**/*.rs` `` → `drift`/`review` sem out-of-scope nos arquivos do fix; `src/**/*.rs` também casou `controller.rs` |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | PASS com ambos `*_ran: true`. `build_cmd: false` → ESCALATE `build_passed: false` sem rodar lint. `lint_cmd: false` → ESCALATE `lint_passed: false` |
| 10 | init lê AGENTS.md/CLAUDE.md e `--write-agents` | **funcionou** | Sem arquivos: `--write-agents` semeou `AGENTS.md` e gravou `.toolbelt/agent_docs.yaml` (`sources: AGENTS.md`). Flag documentada em `tb init --help` |
| 11 | graph Python `calls` edges | **não se aplica** | Repo Rust; não há `.py`. (Observação: index Rust ainda deixa `graph callers`/`impact` de símbolos úteis vazios) |
| 12 | verify visual skipped agregado + docs CLI alinhados | **funcionou** | Manifesto ausente: top-level `skipped: true`. Telas com ref ausente: `skipped: true`, `skip_reason: "todas as telas foram skipped"`. `docs/cli.md` menciona `build_cmd`, `## Scope`, `guard.yaml`, `default_agent`, skip visual |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK (`a46c295`) | neutro | 0,006 s |
| `tb init` / `--write-agents` | OK; `cargo test` + `AGENTS.md` | ajudou | Toolchain Rust detectada; onboarding mínimo criado; ainda não preenche `build_cmd`/`lint_cmd` |
| `tb mem init/list/add/check` | OK | ajudou (parcial) | Invariantes default ainda Go-cêntricas (`tests/hidden/`); `mem add` posicional OK |
| `tb graph rebuild/stats/search` | OK com `--enable` | ajudou | 2981 nós / 4554 arestas / 0,37 s; search achou `largest_offset_from_end` em `line_range.rs` |
| `tb graph impact/callers` | saída vazia em Rust | atrapalhou | `--file src/controller.rs` e `--function largest_offset_from_end` → `(vazio)` |
| `tb graph deps` | OK | neutro | Listou imports de `controller.rs` |
| ticket `rd.md` (`## Escopo`) | manual | ajudou | Parser Markdown + globs `**` consumidos por drift/review |
| `tb drift` | OK | ajudou | Scope do fix limpo; globs `src/**/*.rs` / `tests/**/*.rs` OK (melhoria vs rodada 1) |
| `tb review` | OK / 1 warning completeness | misto | Sem FP de `tests/hidden`; ainda avisa aceite “Existing line-range tests still pass” apesar dos testes no diff |
| `tb verify run` | PASS com build+lint+test+hidden | ajudou | `hidden` sem dir; `build_cmd`/`lint_cmd` efetivos; aborta se `guard.yaml` proteger arquivos do próprio fix |
| `tb verify --ruler visual` | PASS skipped | ajudou | Agregação top-level `skipped: true` (corrigido vs rodada 1) |
| `tb guard check` | OK + lê YAML | ajudou | `protected_paths` custom + defaults; `--strict --json` exit 1 |
| `tb bench run` (`default_agent`) | OK com `-t` absoluto | misto | Honra `default_agent`; **ignora** `bench.tasks_dir` do YAML (sempre default CLI `bench/tasks` sem `-t`) |
| `tb bench compare --agent` / `--fake` | OK | ajudou | Flag `--agent` presente; A/B mcp-smoke 100 %/100 % |
| `tb mcp` | OK (stdio) | ajudou | `tools/list` com version/echo/env/run/graph_*; stderr `servidor MCP iniciado` |

## Problemas que continuam

Da rodada 1 em bat, ainda observáveis:

- **`graph impact` / `callers` vazios em Rust** — search acha o símbolo; impacto/callers não guiam a mudança.
- **Completeness do review frágil** — warning de aceite mesmo com testes adicionados no diff.
- **Invariantes default Go-cêntricas** em `mem init` (`tests/hidden/`, `thresholds.yaml`) num repo Rust.
- **Config files mode `0600`** — `.toolbelt/config.yaml`, `AGENTS.md` gerado, etc.
- **`bench.tasks_dir` do config inerte** sem `-t` na CLI (relacionado ao onboarding vazio de `bench/tasks` local) — na prática o agente precisa passar `-t` apontando para fixtures externas.
- **Verify aborta imediatamente** se `guard.yaml` proteger paths que o próprio fix precisa tocar (comportamento intencional do guard, mas atrapalha o loop verify↔implementação se o manifesto for amplo demais).

## Problemas novos

| sev | problema | repro | esperado | atual |
|-----|----------|-------|----------|-------|
| P2 | `bench.tasks_dir` / `fixtures_dir` do `.toolbelt/config.yaml` ignorados | Setar paths absolutos no YAML; rodar `tb bench run -r 1 --task mcp-smoke` sem `-t` | usar dirs do config | log mostra `tasks_dir: bench/tasks` e falha `open bench/tasks/...` |
| P2 | `tb init` detecta `cargo test` mas não sugere `build_cmd`/`lint_cmd` Rust | `tb init` em bat | preencher `cargo build` / `cargo fmt` ou `clippy` | só `test_cmd: cargo test`; `hidden_test_cmd: ""`; build/lint vazios até edição manual |
| P3 | `graph callers` sem arestas `calls` úteis em Rust (search≠callers) | `tb graph callers --enable --function largest_offset_from_end` | callers a partir de `controller.rs` | `(vazio)` apesar de 994 `edges_calls` agregadas no stats |
| P3 | Review completeness ainda por tokens fracos | Aceite “Existing line-range tests still pass” com testes no diff | sem warning ou evidência via nomes de teste | `accept-1` warning |

Não houve regressão clara das 12 correções neste repo: os itens que falhavam na rodada 1 e estavam no escopo das correções passaram nos testes acima.

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift/scope com `## Escopo` + globs `**` (arquivos do fix dentro do escopo).
- Guard via `guard.yaml` (CHANGELOG/controller) e exit code correto com `--strict --json`.
- Verify falhou corretamente quando `build_cmd` ou `lint_cmd` quebravam; passou com build+fmt+testes focados+hidden sem dir `tests/hidden/`.
- Graph search localizou `largest_offset_from_end`.
- Visual skipped agregado no relatório top-level.

**Deixou passar**
- Impacto real de alterar `controller.rs` (impact/callers vazios).
- Não guiou sozinho o preenchimento de `build_cmd`/`lint_cmd` nem CHANGELOG (ainda veio do CONTRIBUTING humano).
- Completeness do review não reconheceu os testes novos como evidência do aceite.

**Nativo vs tb:** nativo (`build`/`fmt`/`clippy`/testes focados) validou a correção; `tb verify` ecoou os cmds configurados (incluindo hidden sem diretório) e agora também build/lint — melhora clara vs rodada 1.

## Interação com instrumentação existente / onboarding

Não havia instrumentação AI prévia. Com o tb atual:

- `tb init --write-agents` criou `AGENTS.md` mínimo (toolbelt) + `agent_docs.yaml`.
- Detectou `Cargo.toml` → `cargo test`.
- **Não** gerou `CLAUDE.md`, progress/checklist, seed de `bench/tasks` locais, nem `build_cmd`/`lint_cmd` cargo.
- Qualidade do bootstrap: **melhor que a rodada 1** (não mais default Go; há AGENTS.md), ainda **incompleta para Rust** (lint/build e bench local vazios).

## Comparação com a rodada 1

| área | rodada 1 | rodada 2 | veredito |
|------|----------|----------|----------|
| init toolchain | `go test ./...` | `cargo test` | **melhor** |
| AGENTS.md | ausente | `--write-agents` semeia | **melhor** |
| verify build/lint | ignorados | executados e falham o veredicto | **melhor** |
| hidden sem dir | nunca rodava | roda `hidden_test_cmd` | **melhor** |
| guard.yaml | ignorado | lido + strict/json OK | **melhor** |
| Scope `**` / `## Escopo` | frágil / `src/**/*.rs` falhava | casou nos arquivos do fix | **melhor** |
| review FP `tests/hidden` | confirmado | não reproduziu | **melhor** |
| visual skipped top-level | `skipped: false` com telas skipped | `skipped: true` | **melhor** |
| bench default_agent / compare `--agent` | quebrado | funciona | **melhor** |
| graph impact Rust | vazio | ainda vazio | **igual** |
| completeness review | frágil | ainda frágil | **igual** |

No geral: **melhor** de forma concreta nas 12 correções aplicáveis; gaps estruturais de grafo Rust e onboarding lint/build permanecem.

## Sugestões de melhoria

1. **[P1 · init/verify]** Além de `test_cmd`, preencher `build_cmd`/`lint_cmd` por toolchain (`cargo build --bins`, `cargo fmt -- --check` ou `clippy`).
2. **[P1 · bench]** Honrar `bench.tasks_dir` / `fixtures_dir` do config quando as flags CLI estão no default.
3. **[P2 · graph]** Emitir `calls`/`impact` úteis para Rust (hoje search ajuda; callers/impact não).
4. **[P2 · review]** Completeness: casar nomes de teste/arquivos do diff com critérios de aceite, não só substring.
5. **[P3 · mem]** Defaults de invariantes por toolchain (não sempre `tests/hidden/` Go).
6. **[P3 · config]** Permissões 0644 para YAML/AGENTS não-secretos.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time aproximado do estudo | ~25–40 min (upgrade Rust + build bat + fluxo tb + testes focados) | ~25–35 min |
| Invocações `tb` registradas | **~45** (inclui probes intencionais de falha) | 52 |
| Falhas `tb` (exit ≠ 0) | **~12–14** (várias intencionais: strict guard, lint/build fail, bench sem `-t`, review drift de probe) | 13 |
| Soma tempos `tb` (aprox.) | **~10 s** comandos curtos + ~2,3 s verify PASS + ~4,4 s bench fake | ~7,0 s |
| `tb graph rebuild` (bat) | 0,37 s (2981 nós) | 0,26 s (2971 nós) |
| `tb verify run` PASS | ~2,3 s | (não comparável — build/lint agora rodam) |
| `tb bench run --fake` (9 tasks) | ~4,4 s · 33,3 % | ~4,0 s · 33,3 % |
| Binário tb | `a46c295` | `c8aae04` |

Artefatos locais do estudo (somente VM): `/tmp/campo-bat/metrics.tsv`, `/tmp/campo-bat/out/*.txt`, clone `/tmp/target/bat` (não empurrado ao upstream).
