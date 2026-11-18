[🇺🇸 English](../../campo/ruff.md) | 🇧🇷 Português

# Estudo de campo: `tb` em astral-sh/ruff

Data: 2026-11-15. Clone local em `/tmp/target/ruff` (shallow). Nenhuma interação com o remoto do alvo. Alterações de código ficaram só no clone local; neste repositório toolbelt só este arquivo.

`tb` usado: build local `make build` → `./bin/tb` (versão embutida `c8aae04`).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [astral-sh/ruff](https://github.com/astral-sh/ruff) |
| Linguagem | Rust (monorepo: Ruff + ty); Python em fixtures/scripts |
| Tamanho | Checkout shallow ~145–186 MiB (sem `target`); após build local `target/` ~2,3 GiB; ~2055 arquivos `.rs` |
| Instrumentação AI existente | `AGENTS.md` (~20 KiB), `CLAUDE.md` (`@AGENTS.md`), `.agents/` (skills + threat models + plugin Claude), `.claude/settings.json` + hooks |
| Toolchain | **Parcial.** Rust 1.99.0 (via `rust-toolchain.toml`), `cargo`, `cargo-nextest`, `uv` 0.13.8, `cargo-insta` via `uv run`. Compilou e testou o crate `ruff_linter`. **Não** rodou: suíte workspace completa, `cargo clippy --workspace`, hooks `prek`, playground, builds release, testes de file-watcher |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Evitar que `FURB101`/`FURB103` emitam fix “safe” reescrevendo `open(b"...")` → `Path(b"...").read_*()` / `write_*()`, que quebra em runtime (`TypeError` em `pathlib.Path`). Skip de bytes-literal no mesmo ponto em que file descriptors já são ignorados (`helpers.rs`). Fixtures + snapshots atualizados. |
| Por quê | Bug real, escopo pequeno, com testes de snapshot existentes; bloqueia estabilização de preview (`#27051`). |
| Issue | https://github.com/astral-sh/ruff/issues/26922 (comentário pede direção; implementação local seguiu a opção 3 alinhada ao skip de fds em `#27643`) |
| Completa? | **Sim no clone local** (código + fixtures + snapshots; `cargo test -p ruff_linter --lib rules::refurb::tests` → 45 ok). Não enviada ao upstream (proibido pelas regras do estudo). |
| Checks nativos | `cargo test -p ruff_linter --lib rules::refurb::tests` PASS (~87 s frio / ~0,06 s quente). `cargo check -p ruff_linter` PASS no commit do fix (~20 s). Clippy workspace / nextest workspace **não** concluídos. |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | OK; criou `.toolbelt/` + `config.yaml` | atrapalhou (defaults) | `verify.test_cmd` default `go test ./...` num repo Rust; `config.yaml` mode `600` |
| `tb version` (make build) | `toolbelt c8aae04` | neutro | ldflags do Makefile preenchem versão |
| `tb version` (go build sem ldflags) | `0.1.0-dev`, `commit: unknown`, `built: unknown` | atrapalhou | `/tmp/tb-nolags version` |
| `tb mem init/list/add/check` | OK; 3 invariantes default + 1 adicionada | ajudou | `mem add` aceita regra posicional; `check` reportou `compactacao_segura: true` |
| `tb graph rebuild --enable` | 58926 nós, 92578 arestas em ~4,96 s | misto | Indexação rápida; mas só **232** arestas `calls` no monorepo Rust inteiro |
| `tb graph search --enable read_whole_file` | Achou `read_whole_file`, `ReadWholeFile`, etc. (~0,37 s) | ajudou | Localizou arquivos da regra |
| `tb graph impact --enable --file …/helpers.rs` | vazio / `null` | atrapalhou | Sem impacto útil apesar do arquivo existir no grafo |
| `tb graph impact` sem `--file` | erro `--file é obrigatório` | neutro | Confirma flag obrigatória |
| `tb graph callers --enable --function find_file_open` | vazio | atrapalhou | Call graph Rust fraco |
| `tb graph deps --enable --file …/helpers.rs` | listou pkgs regex-ish (`pkg:crate::…`) | neutro | Dependências superficiais |
| ticket / Scope (`rd.md` manual) | `tb ticket` **não existe** no binário | atrapalhou | Docs descrevem `tb ticket`; CLI real: unknown command. Scope via frontmatter YAML funciona |
| `tb drift --ticket rd.md` | Detectou `rd.md` fora do scope | ajudou | JSON com `out_of_scope: ["rd.md"]`; base batendo com `base_commit` |
| `tb drift` com globs | `/**` (sufixo) e `dir/*.py` funcionam; paths literais também | misto | `rd-glob2.md` / `rd-glob3.md`; sem glob, literais limpos em `rd-literal.md` |
| `tb verify run --ruler tests` | PASS no fix (~0,22 s quente) com `cargo test -p ruff_linter …` | misto | `hidden_available: false` (não há `tests/hidden/`); defaults Go inúteis sem reconfig |
| `tb verify` + erro de build | Com `--test-cmd true` → PASS; com `cargo check` → ESCALATE | atrapalhou | Verify só roda o comando configurado; não força build |
| `tb verify run --ruler visual` (sem manifesto) | PASS, `skipped: true`, motivo manifesto ausente | neutro | Skip gracioso no relatório top-level |
| `tb verify` visual com manifesto sem refs | PASS, **`skipped: false`**, tela `skipped: true` | atrapalhou | Relatório top-level não marca skip quando só as telas skipam |
| `tb review` | Findings: scope `rd.md` + FP invariante `tests/hidden` | atrapalhou | Diff do ticket menciona `tests/hidden/` → falso positivo crítico; completeness por tokens fraca |
| `tb guard check HEAD~1..HEAD` | Sem violações no fix | neutro | UX diferente (`base..head`, não `--base`) |
| `tb guard` + `.toolbelt/guard.yaml` custom | Custom **ignorado**; paths hardcoded | atrapalhou | Código: `NewChecker` fixa `tests/hidden/*`, `benchmarks/*`, `.toolbelt/thresholds.yaml` |
| `tb guard` tocando `thresholds.yaml` | Violação `protected_path`; `verify` retorna FAIL sem rodar testes | misto | Abort esperado; path hardcoded atrapalha se o agente mexe em config tb |
| `tb bench run` sem `--agent`/`--fake` | Erro obrigatório, ignora `default_agent` | atrapalhou | Config com `default_agent: "echo-agent {prompt}"` ignorada |
| `tb bench run --fake` | OK sem `--fake-config` | ajudou | 9 tasks / 1 rep no repo toolbelt; sucesso 33,3% |
| `tb bench compare` | Sem flag `--agent` | neutro/confirmado | Só `--fake` / JSONL / configs A/B |
| `tb mcp` | `tools/list` + `tools/call` (`tb_version`, `tb_graph_search`) OK | ajudou | Com `TB_GRAPH=on`, search retornou `ReadWholeFile` |
| Docs `cli.md` vs binário | `ticket`, `board`, `run`, `md` documentados, ausentes no CLI | atrapalhou | `tb ticket` / `tb board` / `tb run` → unknown command |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --help`: sem `--agent`; só `--fake` para A/B |
| `guard.yaml` ignored | **confirmado** | Arquivo `.toolbelt/guard.yaml` criado; checker não lê; docs ainda documentam; só paths hardcoded em `internal/guard/guard.go` |
| outdated CLI docs | **confirmado** | `docs/cli.md` lista `tb ticket`, `tb board`, `tb run`, `tb md`; binário Fase 2 não tem |
| version shows `unknown` | **parcialmente confirmado** | Sem ldflags: versão `0.1.0-dev`, **commit/built = unknown**. Com `make build`: versão = short hash |
| config files created 0600 | **confirmado** | `stat`: `config.yaml` e `invariants.yaml` mode `600` |
| visual ruler reports `skipped=false` when skipping | **confirmado** | Manifesto presente, refs ausentes: top-level `"skipped": false` com screen `"skipped": true` / `skip_reason: referência ausente` |
| `default_agent` ignored | **confirmado** | `bench.default_agent` setado; `tb bench run` ainda exige `--agent` ou `--fake` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | Commit em `.toolbelt/thresholds.yaml` → guard error → verify `FAIL` `iterations: 0` |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake` funciona sem `--fake-config` (agente fake embutido) |
| `mem add` positional | **confirmado** | Usage: `tb mem add <regra>`; sem `--rule`; join dos args posicionais |
| Drift/Scope need literal paths (no globs) | **parcialmente confirmado** | Literais OK. Sufixo `/**` e `filepath.Match` (`dir/*.py`) OK. Docs/UX ainda empurram literais; globs no meio do path/`**` genérico limitados |
| `graph impact` needs `--file` | **confirmado** | Sem `--file`: `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | **confirmado** | Commit do ticket com `forbidden: tests/hidden/` no diff → finding critical mesmo sem tocar em `tests/hidden/` |
| verify runs only partial checks and misses build errors | **confirmado** | Defaults Go; `hidden` só se existir dir; `--test-cmd true` PASS com Rust quebrado; só falha se `test_cmd` incluir build |
| tb does not push agent toward completing the whole task | **confirmado (neste run)** | Sem `tb ticket`/`board`; init não importa `AGENTS.md`/skills; review completeness = matching fraco de tokens; ruído de FP/scope distrai; verify defaults não guiam ao checklist do repo |

## Problemas novos

1. **P1 — Call graph / impact inúteis em monorepo Rust**  
   - Repro: `TB_GRAPH=on tb graph rebuild --enable` depois `tb graph impact --enable --file crates/ruff_linter/src/rules/refurb/helpers.rs` → vazio; `callers --function find_file_open` → vazio. Stats: 31116 nós `.rs`, mas só 232 edges `calls`.  
   - Esperado: impacto/callers a partir de funções Rust.  
   - Atual: indexação de símbolos/arquivos OK; relações `calls` quase ausentes.

2. **P2 — Defaults de `verify` Go-cêntricos sem detecção de linguagem**  
   - Repro: `tb init` em ruff → `test_cmd: go test ./...`.  
   - Esperado: detectar Cargo/`AGENTS.md` e sugerir comandos do projeto.  
   - Atual: agente precisa reconfigurar manualmente ou verify “passa” sem significado.

3. **P2 — UX inconsistente guard vs drift/review**  
   - Repro: `tb guard check --base HEAD~1` → unknown flag; precisa `tb guard check HEAD~1..HEAD`.  
   - Esperado: mesmas flags `--base`/`--head`.  
   - Atual: dois dialetos.

4. **P3 — `graph callers`/`deps` exigem flags, não posicionais**  
   - Repro: `tb graph callers --enable find_file_open` falha (`--function` obrigatório).  
   - Esperado (pelo hábito CLI): argumento posicional ou mensagem amigável.  
   - Atual: erro de usage fácil de tropeçar no fluxo agente.

5. **P3 — Completeness do review não valida aceite de verdade**  
   - Repro: critérios de aceite no ticket; review não falhou completeness apesar de critérios comportamentais (“no longer triggers”).  
   - Esperado: checagem ligada a testes/`verify`.  
   - Atual: heurística de tokens no diff (`reviewCompleteness`).

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de `rd.md` fora do scope declarado.
- Base commit alinhada ao ticket.
- Guard em path protegido `.toolbelt/thresholds.yaml`.
- Verify com `cargo check` detectou build quebrado (ESCALATE após 3 iterações).
- Graph search achou símbolos FURB101.
- MCP expôs as mesmas ferramentas básicas.

**Deixou passar / atrapalhou**
- Não leu `AGENTS.md` nem skills (comandos de teste, política de snapshots, AI policy).
- Defaults `go test` não teriam exercitado o fix.
- Review: falso positivo crítico em `tests/hidden` e ruído de scope no próprio ticket.
- Impact/callers não ajudaram a achar efeitos colaterais.
- Com `test_cmd` fraco, verify PASS com código Rust inválido.
- Não guiou estabilização preview / discussão de design da issue (3 opções no comentário).
- Não sugeriu rodar clippy/nextest workspace.

## Interação com instrumentação existente / onboarding

- **`tb` ignora** `AGENTS.md`, `CLAUDE.md`, `.agents/skills/*`, threat models e hooks `.claude/`. Nenhum código Go referencia esses paths (só docs de pesquisa mencionam AGENTS.md como ideia futura de `tb ctx`).
- **`tb init`** cria vault/invariantes/config **paralelos**, sem importar regras do repo (ex.: “nunca editar snapshots à mão”, filtros `file_watching`, `INSTA_*`, política de AI).
- **Duplicação potencial**: invariantes genéricas (`tests/hidden/`, `thresholds.yaml`) não batem com o harness real do Ruff (mdtest/insta/nextest) e geram FPs.
- **Conflito**: defaults Go vs instruções Cargo do `AGENTS.md`.
- **O que o tb deveria importar**: (1) blocos de “Running Tests” / Clippy do `AGENTS.md` → `verify.test_cmd` / lint; (2) skills `.agents/skills` como playbooks opcionais; (3) proibições de snapshot manual → `mem` invariantes; (4) labels/PR conventions só se houver fluxo de PR (aqui não aplicável).

Onboarding observado: `tb init` + `tb mem init` em segundos, mas **não** onboa o agente no workflow do Ruff; o agente humano/AI ainda precisa ler `AGENTS.md` por fora.

## Sugestões de melhoria

1. **[graph/P1]** Indexer Rust com `calls`/`impact` reais (ou documentar limitação e não anunciar impact em Rust até existir).  
2. **[verify/P1]** Detectar toolchain no `init` (Cargo.toml → `cargo test -p …`; ler `AGENTS.md` se presente) e incluir `cargo check`/`clippy` no preset.  
3. **[mem-ctx/P1]** `tb init --import-agents` (ou automático): extrair invariantes e test cmds de `AGENTS.md` / `.agents`.  
4. **[review/P1]** FP `tests/hidden`: só acionar se path **alterado** estiver sob `tests/hidden/`, não se a string aparecer no ticket.  
5. **[guard/P2]** Honrar `.toolbelt/guard.yaml`; alinhar flags `--base`/`--head`.  
6. **[docs/P2]** Sincronizar `docs/cli.md` com comandos reais; marcar ticket/board/run/md como planejados.  
7. **[bench/P2]** Respeitar `default_agent`; opcionalmente `--agent` em `compare`.  
8. **[verify/P2]** Relatório visual: `skipped=true` no top-level se todas as telas skiparam.  
9. **[cli/P3]** Aceitar posicionais em `graph callers`/`deps`/`impact` além das flags.  
10. **[router/ticket/P2]** Implementar ou remover docs de `tb ticket`/`board` — sem isso o tb não empurra fechamento de aceite ponta a ponta.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time do estudo (aprox., após build tb) | ~6–15 min de uso tb+cargo no alvo; graph rebuild 4,96 s; cargo test frio refurb ~87 s; verify quente ~0,22 s; verify+cargo check quebrado ~19 s |
| Invocações tb (famílias / marcações de log) | ~25 famílias de comando; ~36 seções `===` no log `/tmp/tb-campo-ruff.log` |
| Falhas / erros tb observados | `graph impact` sem `--file`; `callers`/`deps` sem flags; `ticket`/`board`/`run` inexistentes; `bench run` sem agent; `drift`/`review` exit≠0 por findings; verify ESCALATE com build quebrado; guard unknown `--base` |
| Crashes tb | Nenhum crash/segfault observado |
| MCP | `tools/list` + `tb_version` + `tb_graph_search` OK via stdio |

Log bruto local (VM): `/tmp/tb-campo-ruff.log`. Clone e commits de estudo **não** foram enviados a `astral-sh/ruff`.
