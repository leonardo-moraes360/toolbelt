[🇺🇸 English](../../campo/bat.md) | 🇧🇷 Português

# Estudo de campo: sharkdp/bat

Grupo: **sem instrumentação de agente de IA**. Clone local em `/tmp/target/bat`. Nenhuma interação com o upstream (sem push/issue/PR/comentário). `tb` buildado deste repo (`make build` → `toolbelt c8aae04`).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [sharkdp/bat](https://github.com/sharkdp/bat) |
| Linguagem | Rust (edition 2021, MSRV declarado `1.88`) |
| Tamanho | shallow clone ~15–17 MiB código; ~77 arquivos `.rs` / ~21 515 LOC; `target/` local ~1,9 GiB após build |
| HEAD estudado | `599c383` (merge #3837) + commit local de correção `af23f13` (somente no VM) |
| Instrumentação AI existente | **Nenhuma** (`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, instruções Copilot: ausentes) |
| Toolchain | Rustup `stable` 1.100.0 (inicialmente 1.83.0 — insuficiente para MSRV); `cargo build --bins`, `cargo test` (integration), `cargo fmt`, `cargo clippy --bins` OK |
| Não configurado | Submódulos de `assets/syntaxes/*` e temas **não** inicializados (checkout já traz assets pré-compilados `acknowledgements.bin` etc.; regeneração via `assets/create.sh` não foi necessária nem executada). Suite completa de syntax-tests / snapshots não foi rodada por tempo e dependências de assets. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Validar `--language` / `--map-syntax` mesmo com highlighting desligado (`--color=never` / pipe), falhando com `unknown syntax` e exit ≠ 0 |
| Por quê | Bug real, escopo pequeno-médio, aceitável por maintainer, com caminho claro de teste de integração |
| Issue | https://github.com/sharkdp/bat/issues/4120 |
| Completada? | **Sim** (localmente): `src/controller.rs` + validação espelhada em `src/printer.rs`, 2 testes em `tests/integration_tests.rs`, entrada no `CHANGELOG.md` |
| Checks nativos | `cargo build --bins` OK; `cargo fmt -- --check` OK; `cargo clippy --bins -- -D warnings` OK; testes `unknown_language_errors_with_color_never` e `unknown_map_syntax_errors_with_color_never` OK; regressão `no_first_line_fallback_when_mapping_to_invalid_syntax` OK. Suite integration completa (~290 testes) **não** foi executada por completo. |

Causa raiz: com `loop_through` (color never / pipe), `Controller` usa `SimplePrinter` e **pula** `get_syntax`, então nomes inválidos nunca são rejeitados.

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK (`c8aae04` com `make build`) | neutro | 0,006 s; sem ldflags: `commit: unknown` / `built: unknown` |
| `tb init` | OK; cria `.toolbelt/` | atrapalhou (parcial) | `verify.test_cmd: go test ./...` num repo Rust; sem `AGENTS.md`; dirs `vault/` e `bench/` vazios; `config.yaml` mode **600** |
| `tb mem init/list/add/check` | OK | ajudou (parcial) | invariantes default Go-cêntricas (`tests/hidden/`, `thresholds.yaml`); `mem add` só aceita **posicional** (`--rule` → unknown flag); arquivos 600 |
| `tb graph rebuild/stats/search` | OK com `--enable` / `TB_GRAPH=on` | ajudou | 2971 nós / 4323 arestas em 0,26 s; `search get_syntax` achou `src/assets.rs` |
| `tb graph impact` | exige `--file`; com arquivo → saída vazia | atrapalhou | sem `--file`: exit 1; `--file src/printer.rs`: `(vazio)` — impacto inútil para guiar a mudança |
| `tb graph callers/deps` | OK só com flags corretas | neutro | sem `--function`/`--file` falha; callers de `get_syntax` vazio; deps de `src/printer.rs` listou imports |
| ticket + Scope (`rd.md`) | manual (não há `tb ticket`) | ajudou | frontmatter `base_commit`/`scope`/`forbidden`/`acceptance` consumido por drift/review |
| `tb drift` | OK / detecta fora de escopo | ajudou | flagrou `rd.md` fora do scope; globs simples `src/*.rs` e `tests/**` funcionam; `src/**/*.rs` **não** |
| `tb review` | OK / findings | misto | drift+aceite; FP de invariante `tests/hidden` quando o diff menciona a string; avisos de completeness mesmo com testes no diff |
| `tb verify run` (tests) | PASS se `test_cmd` válido | misto | ignora `build_cmd`/`lint_cmd` do YAML; `hidden_test_cmd` só roda se existir dir `tests/hidden/`; com `test_cmd=true` PASS em commit que **não compila** |
| `tb verify --ruler visual` | PASS com skip | atrapalhou | manifesto ausente: `skipped=true`; com telas sem referência: top-level **`skipped=false`** embora screen `skipped=true` |
| `tb guard check` | OK | misto | não lê `.toolbelt/guard.yaml`; paths hardcoded (`thresholds.yaml` etc.); aborta `verify` ao tocar path protegido |
| `tb bench run --fake` | OK | neutro | 9 tasks, 33,3 % sucesso, ~4 s; `default_agent` no YAML **ignorado** (exige `--agent`/`--fake`) |
| `tb bench compare` | OK com `--fake` A/B | neutro | **não** tem `--agent` (`unknown flag`); A/B graph-off/on em mcp-smoke OK |
| `tb mcp` | OK (stdio JSON-RPC) | ajudou | `tools/list` expôs version/echo/env/run/graph_*; stderr: `servidor MCP iniciado` |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|----------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --agent x …` → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmado** | `.toolbelt/guard.yaml` listando `CHANGELOG.md`/`src/controller.rs`; `guard check` → `violations: null` |
| outdated CLI docs | **confirmado** | `docs/cli.md` documenta `tb run`, `tb proc`, `tb md`, `tb ticket`; `tb --help` real não os lista |
| version shows `unknown` | **confirmado** (parcial) | `go build` sem ldflags: `commit: unknown`, `built: unknown` (versão `0.1.0-dev`); com `make build` mostra hash |
| config files created 0600 | **confirmado** | `stat`: `.toolbelt/config.yaml` e `invariants.yaml` e `graph.json` → mode **600** |
| visual ruler reports `skipped=false` when skipping | **confirmado** | com `screens.yaml` e ref ausente: JSON top-level `"skipped": false` enquanto screen tem `"skipped": true` |
| `default_agent` ignored | **confirmado** | YAML `default_agent: echo-agent {prompt}`; `tb bench run` sem flags → `--agent ou --fake é obrigatório` |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | commit tocando `.toolbelt/thresholds.yaml` → verify `FAIL` iterações 0, violation `protected_path` |
| default fake agent needs `--fake-config` | **refutado** | `tb bench run --fake … --task mcp-smoke` exit 0 **sem** `--fake-config`; path inválido em `--fake-config` também foi ignorado com sucesso |
| `mem add` positional | **confirmado** | Usage: `tb mem add <regra>`; `--rule` → `unknown flag` |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `src/*.rs`, `src/print*.rs`, `tests/**`, `CHANGELOG.*` OK; `src/**/*.rs` e `**/*.rs` **falham** (todos os `.rs` fora do scope) |
| `graph impact` needs `--file` | **confirmado** | sem flag: `Error: --file é obrigatório` |
| review false positives on the tests/hidden invariant | **confirmado** | com `rd.md` no diff contendo `tests/hidden/`: finding critical na invariante; sem essa string no diff: FP desaparece |
| verify runs only partial checks and misses build errors | **confirmado** | `build_cmd`/`lint_cmd` no YAML não executados; commit com Rust inválido + `--test-cmd true` → **PASS**; `cargo build` nativo exit 101 |
| tb does not push the agent toward completing the whole task | **confirmado** (qualitativo) | onboarding não gerou checklist/progress/`AGENTS.md` nem detectou `cargo test`/`clippy`; `review` só avisou completeness por substring no diff; CHANGELOG/testes vieram do CONTRIBUTING humano, não do tb |

## Problemas novos

| sev | problema | repro | esperado | atual |
|-----|----------|-------|----------|-------|
| P1 | `tb init` assume Go em repo Rust | `tb init` em bat | detectar `Cargo.toml` → `cargo test` / `cargo clippy` | `verify.test_cmd: go test ./...` |
| P1 | `hidden_test_cmd` CLI/config ignorado sem dir `tests/hidden/` | `tb verify run --hidden-test-cmd 'cargo test … basic'` | rodar comando oculto | `hidden_available: false`; nunca executa o cmd |
| P2 | `graph impact` vazio em Rust | `tb graph impact --enable --file src/printer.rs` | callers/dependentes | `(vazio)` |
| P2 | UX cobra flags sem hint no erro curto de deps/callers | `tb graph callers get_syntax` | aceitar posicional ou sugerir `--function` | exit 1 genérico |
| P2 | Completeness do review é frágil (substring) | critérios de aceite com `--map-syntax` / “unchanged” | evidência via testes/changelog | warnings mesmo com testes adicionados |
| P3 | `bench.default_agent` no schema sem efeito | setar no YAML e omitir CLI | usar default | erro obrigando `--agent`/`--fake` |
| P3 | MCP `tools/list` inclui tools de graph mesmo com graph off (docs dizem sob demanda) | `tb mcp` sem `TB_GRAPH` | omitir ou anotar disabled | tools graph listadas; busca pode falhar depois |

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift quando `rd.md` entrou no commit fora do Scope.
- Guard em path hardcoded `.toolbelt/thresholds.yaml` (e abortou verify).
- Review: avisos de critérios de aceite pouco evidentes no texto do diff.
- Graph search localizou `get_syntax` / arquivos relacionados.

**Deixou passar**
- Erro de build Rust com `verify` configurado só com `test_cmd` (ou `true`).
- `build_cmd` / `lint_cmd` declarados no config.
- Path “protegido” via `guard.yaml` do usuário.
- Impacto real de mudar `printer.rs`/`controller.rs` (impact vazio).
- Não guiou CHANGELOG (exigência do CONTRIBUTING do bat) nem suite completa.

**Nativo vs tb:** nativo (`build`/`fmt`/`clippy`/testes focados) validou a correção; tb verify só ecoou o `test_cmd` que eu configurei manualmente após corrigir o default Go.

## Interação com instrumentação existente / onboarding

Não havia instrumentação AI prévia. `tb init` + `tb mem init` a partir do zero geraram:

- `.toolbelt/config.yaml` (defaults sandbox/graph/bench/verify **Go**)
- `.toolbelt/invariants.yaml` (anti-cheat genérico)
- dirs vazios `vault/`, `bench/`, `logs/`
- depois: `graph.json` (via rebuild), mais `guard.yaml`/`screens.yaml` criados manualmente no estudo

**Ausente e sentido falta:** `AGENTS.md` / skills do repo; detecção automática de `cargo test`/`clippy`/`fmt`; progress log / feature list no estilo harness; ticket CLI (`tb ticket`); seed de `bench/tasks` locais; documentação curta “como operar neste repo”.

Qualidade do bootstrap: **baixa para Rust sem AI files** — o agente precisa sobrescrever quase todo o verify e ignorar invariantes irrelevantes.

## Sugestões de melhoria

1. **[P0 · init/verify]** Detectar toolchain (`Cargo.toml`, `go.mod`, `package.json`) e preencher `test_cmd`/`build_cmd`/`lint_cmd`; falhar alto se default Go em repo não-Go.  
2. **[P0 · verify]** Executar `build_cmd` e `lint_cmd` no loop (ou pelo menos no veredicto final); não dar PASS só com `test_cmd`.  
3. **[P1 · verify]** Honrar `--hidden-test-cmd` / config mesmo sem diretório `tests/hidden/`.  
4. **[P1 · guard]** Carregar `.toolbelt/guard.yaml` (ou documentar que é morto e remover).  
5. **[P1 · bench]** Usar `bench.default_agent` quando `--agent` omitido; adicionar `--agent` em `bench compare` A/B.  
6. **[P1 · onboarding]** Gerar `AGENTS.md` mínimo + progress/checklist a partir do ticket; empurrar aceite (testes + changelog se o repo exigir).  
7. **[P2 · review]** Matching de invariantes por path/glob, não substring no diff (evita FP com `tests/hidden` no texto do ticket).  
8. **[P2 · visual]** Se qualquer screen for skip, `report.skipped=true` (ou `partial_skip`).  
9. **[P2 · graph]** Impact/callers úteis em Rust; aceitar posicionais ou erros acionáveis.  
10. **[P3 · config]** Permissões 0644 para YAML não-secretos; alinhar `docs/cli.md` ao Cobra real; ldflags/`version` consistentes no README.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time aproximado do estudo | ~25–35 min (upgrade Rust + build bat + fluxo tb + testes focados); soma instrumentada de comandos ~22 s |
| Invocações `tb-*` registradas | **52** |
| Falhas `tb-*` (exit ≠ 0) | **13** (várias intencionais: flags ausentes, drift/review, guard abort, compare `--agent`) |
| Soma tempos `tb-*` | **~7,0 s** |
| Build inicial bat | ~30,6 s |
| `tb graph rebuild` | 0,26 s (2971 nós) |
| `tb bench run --fake` (9 tasks) | ~4,0 s |
| MCP `tools/list` | ~0,006 s |

Artefatos locais do estudo (somente VM): `/tmp/campo-bat/metrics.tsv`, JSONs de verify/review/drift, clone `/tmp/target/bat` (não empurrado).
