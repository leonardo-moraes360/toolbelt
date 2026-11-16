# Campo rodada 2: BurntSushi/ripgrep

Estudo de campo ROUND 2 do `tb` (grupo: **without existing AI-agent instrumentation**). Clone local apenas — sem push/issue/PR/comentário no upstream.

- **tb build**: `make build` em `main` → `toolbelt a46c295` (commit `a46c295`, built `2026-11-16T12:42:05Z`)
- **Alvo**: clone shallow em `/tmp/target/ripgrep` (HEAD inicial `178cc02`)
- **Janela**: ~2026-11-16T12:35Z → 12:43Z UTC
- **Evidências**: `/tmp/tb-campo2-ripgrep/`

---

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) |
| Linguagem | Rust (edition 2024 / `rust-version` 1.96; workspace `grep-*`) |
| Tamanho | ~104 `.rs` / ~55k LOC; shallow ~4.8 MiB; com `target/` ~676 MiB após build |
| Instrumentação AI existente | Só `AI_POLICY.md` (+ menção em `CONTRIBUTING.md`). Sem `AGENTS.md`/`CLAUDE.md`/`.cursorrules`/`.cursor/` no tree original. Política proíbe agentes autônomos no upstream; estudo 100% local. |
| Toolchain | Rustup `stable` **1.100.0**. `cargo test --tests` (114 + 332) e `cargo build` ok. `rustfmt`/`clippy` disponíveis. Não rodei matrix CI multi-target/`cross`/PCRE2; `cargo clippy -D warnings` workspace-wide não foi a porta principal deste run. |

---

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Evitar `Path::canonicalize` em `StandardSink::start_hyperlink` quando o formato de hyperlink está vazio (#3594) |
| Por quê | Bug aberto no master; escopo pequeno em `grep-printer` com regressão clara; diferente da rodada 1 |
| Issue | https://github.com/BurntSushi/ripgrep/issues/3594 (há PR #3596 fechado **sem** merge — fix ainda ausente em `178cc02`) |
| Completou? | **Sim, só local** (commit `69fedaf` em `/tmp/target/ripgrep`; **não** enviado ao GitHub) |
| Resultado nativo | `cargo test -q -p grep-printer --lib` → 110 ok (inclui `empty_hyperlink_skips_path_canonicalization`); `cargo test -q --tests` → 114 + 332 ok; `cargo fmt --check` ok |
| Rodada 1 (comparação) | #2779 — números de linha errados com `-U`+`-r` em matches multilinha adjacentes |

Arquivos tocados: `crates/printer/src/standard.rs`, `crates/printer/src/util.rs` (+ accessor `#[cfg(test)]` no cache de hyperlink).

---

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|---------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph index --enable /tmp/target` → 2544 nós / 3221 arestas com paths `ripgrep/...` (root `/tmp/target`). Na rodada 1 o mesmo path rendia 0 nós. |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; `--hidden-test-cmd false` → `hidden_available: true`, `hidden_passed: false`, veredicto `ESCALATE` exit 1. Com cmd real de teste → `hidden_passed: true` no PASS final. |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff em `.toolbelt/thresholds.yaml` + `--strict --json` → JSON com `protected_path` e **EXIT=1**. |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `crates/core/main.rs`; diff tocando esse path → `violations: [{type: protected_path, file: crates/core/main.rs}]`, `--strict --json` EXIT=1. |
| 5 | init detecção de toolchain | **funcionou** | `tb init` → `verify.test_cmd: cargo test` (não mais `go test ./...`). |
| 6 | review `tests/hidden` casa por path | **funcionou** | Ticket com texto “Não tocar em tests/hidden/” + diff só em printer → `findings: []` (sem critical por substring). |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | Com `bench.default_agent: echo AGENT_OK {prompt}` no `.toolbelt/config.yaml`, `tb bench run -r 1 --task bugfix-go` (sem `--agent`/`--fake`) → 100% sucesso; `tb bench compare ... --agent 'echo COMPARE_AGENT {prompt}'` aceitou a flag e gerou relatório A/B. |
| 8 | review/drift `## Scope`/`## Escopo`, listas MD e `**` globs | **funcionou** | `## Escopo` + bullets → review sem `scope-missing`; `## Escopo` errado (`crates/core/main.rs`) → drift lista printer files `out_of_scope` EXIT=1; `crates/printer/src/**/*.rs` manteve files in-scope. |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Config com `cargo build`/`cargo fmt --check` → `build_ran/lint_ran: true` no PASS. `build_cmd: false` → `ESCALATE`, `build_passed: false`. |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** | Sem instrumentação: `tb init --write-agents` semeou `AGENTS.md` + `.toolbelt/agent_docs.yaml` (`sources=AGENTS.md`). `CLAUDE.md` ausente (ok). Não importou `AI_POLICY.md`. |
| 11 | graph Python `calls` edges | **não se aplica** | Repo Rust; após index, edge types só `defines`/`imports`/`mentions`/`links` — **0** `calls` (esperado: fix é para Python). |
| 12 | verify visual skipped agregado + docs CLI alinhados | **funcionou (parcial)** | Visual sem manifesto: `skipped: true`, `skip_reason: manifesto de telas ausente` (mas `verdict: PASS` ainda). Docs: `default_agent`, `guard.yaml` e `tb ticket` (marcado Fase 3+) alinhados; ainda há ruído residual vs CLI em exemplos antigos de `mem`/manifest flags. |

---

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` / `--write-agents` | `.toolbelt/` + `test_cmd: cargo test`; seed `AGENTS.md` | **ajudou** | Onboarding Rust ok; `AI_POLICY.md` ainda ignorado |
| `tb mem init/add/list/check` | 3 anti-cheat + 1 custom; `retencao: 1.00` | neutro | Invariantes genéricas; `mem add --type` ainda rejeitado (docs legadas) |
| `tb graph index --enable /tmp/target` | 2544 nós / 3221 arestas | **ajudou** | SkipDirs na raiz corrigido |
| `tb graph search --enable hyperlink` | Achou `HyperlinkFormat`, `as_hyperlink`, etc. | **ajudou** | Acelerou achar `standard.rs` |
| `tb graph impact --enable --file …` | `(vazio)` / JSON `null` | **atrapalhou** | Continua sem blast radius em Rust |
| `tb graph callers/deps` | callers exige `--function` (sem `--file`); deps lista pkgs | neutro→atrapalhou | UX ainda pouco intuitiva |
| Ticket `rd.md` (`## Escopo` + globs) | Parse ok no review/drift | **ajudou** | Contraste forte com rodada 1 |
| `tb verify run` (build/lint/test/hidden) | `PASS`; hidden ran | **ajudou** | Pipeline completo; `hidden_available: true` sem dir |
| `tb verify --ruler visual` | `PASS` + `skipped: true` | neutro | Skip honesto no campo `skipped`; veredicto PASS ainda confunde |
| `tb review` | `findings: []` com scope MD; sem FP `tests/hidden` | **ajudou** | |
| `tb drift` | Detecta fora de `## Escopo`; globs `**` ok | **ajudou** | EXIT≠0 + lista `out_of_scope` |
| `tb guard check` | Limpo no fix; honra `guard.yaml`; `--strict --json` EXIT≠0 | **ajudou** | |
| `tb bench run/compare` | `default_agent` e `--agent` ok; fixtures do toolbelt | neutro | Não mede o ripgrep em si |
| `tb mcp` | `tools/list`: env/run/graph_*/version/echo | neutro | Sem review/verify/drift/guard/mem |
| `tb ticket` | `unknown command` | neutro | Docs agora marcam como planejado |

---

## Problemas que continuam

Da rodada 1 (`docs/campo/ripgrep.md`) que **ainda acontecem** com `tb@a46c295`:

1. **`graph impact` vazio em Rust** — `tb graph impact --enable --file crates/printer/src/standard.rs` → vazio/`null` (igual rodada 1).
2. **Sem arestas `calls` em Rust** — índice só `defines`/`imports`/`mentions`/`links`; callers/impact continuam fracos fora de Go/JS/Python.
3. **Visual: `verdict: PASS` com `skipped: true`** — manifesto ausente; fácil ler como “visual ok”.
4. **MCP << CLI** — sem tools de review/verify/drift/guard/mem/bench.
5. **Configs mode `0600`** — `config.yaml` e `AGENTS.md` semeado ficaram `600`.
6. **`tb` não empurra completude sozinho** — melhor bootstrap, mas a tarefa (#3594) ainda exige iniciativa humana para issue/código/teste.
7. **Não lê `AI_POLICY.md`** — política humana do repo continua invisível ao harness (só relevante se alguém fosse contribuir no upstream; não contribuímos).

Problemas da rodada 1 **corrigidos neste run**: init Go-default; Scope markdown; guard.yaml ignorado; `default_agent`; review FP `tests/hidden`; verify sem build/lint; hidden gated por dir; grafo mudo em `/tmp/target`; ausência de `--write-agents`.

---

## Problemas novos

1. **P2 — `verdict: PASS` no ruler visual quando `skipped: true`**  
   - Repro: `tb verify run --ruler visual --skip-unavailable` sem `.toolbelt/screens.yaml`.  
   - Esperado: veredicto `SKIP`/`N/A` (ou não-PASS) alinhado a `skipped`.  
   - Atual: JSON `skipped: true` + `verdict: PASS`.

2. **P2 — flag global `-c/--config` não alimenta `config.Load()`**  
   - Repro: `tb -c /tmp/.../config.yaml bench run -r 1` com `default_agent` só nesse arquivo → ainda exige `--agent`/`--fake`.  
   - Esperado: `-c` sobrescreve o YAML carregado.  
   - Atual: só `.toolbelt/config.yaml` do cwd / global; a flag parece decorativa.

3. **P3 — `graph impact` permanece silencioso (sem mensagem “sem impacto”)**  
   - Repro: após index Rust rico, impact no printer → `(vazio)`.  
   - Esperado: pelo menos diagnóstico (“0 edges” / “linguagem sem calls”).  
   - Atual: silêncio; search encontra símbolos no mesmo arquivo.

4. **P3 — seed `AGENTS.md` mode 0600 e genérico**  
   - Repro: `tb init --write-agents` em ripgrep.  
   - Esperado: 0644 para doc não-secreta; opcionalmente citar `cargo test`/`AI_POLICY.md`.  
   - Atual: stub toolbelt-centric; não referencia a política do projeto.

Nenhuma regressão óbvia causada pelas 12 correções foi observada nos caminhos exercitados (guard/verify/review/drift/init/graph root).

---

## O que o tb pegou e o que deixou passar

**Pegou**
- Toolchain Cargo no init; seed `AGENTS.md` sob `--write-agents`.
- Graph index em `/tmp/target` + search de hyperlink → `standard.rs`.
- Scope markdown + globs no review/drift; FP de `tests/hidden` eliminado.
- Verify com build/lint/test/hidden (hidden sem pasta) → PASS no fix; ESCALATE se hidden/`build_cmd` falham.
- Guard com `guard.yaml` custom + exit code estrito com JSON.
- Bench `default_agent` / `compare --agent`.

**Deixou passar / não ajudou**
- Não sugeriu a causa raiz (#3594) nem apontou o early-return no hyperlink.
- `graph impact` não guiou blast radius (path.rs/summary.rs também chamam `as_hyperlink`).
- Visual PASS skipped; MCP incompleto; `AI_POLICY.md` ignorado.
- Clippy workspace-wide / matrix CI do ripgrep não entraram no harness.

**Nativo vs tb**: a regressão nova foi vista por `cargo test -p grep-printer`; o tb espelhou isso via `test_cmd`/`hidden_test_cmd` configurados.

---

## Interação com instrumentação existente / onboarding

- **Existente**: apenas `AI_POLICY.md` (proíbe agentes autônomos no upstream).
- **`tb init --write-agents` gerou**: `.toolbelt/{config.yaml,vault,logs,bench}`, `AGENTS.md` stub, `agent_docs.yaml`. Depois `mem init` → `invariants.yaml`; graph → `graph.json`; `guard.yaml` criado manualmente para o teste.
- **Qualidade**: bem melhor que a rodada 1 (`cargo test` automático + AGENTS). Ainda genérico; não importa CONTRIBUTING/AI_POLICY nem sugere `cargo fmt`/`clippy` no seed.
- **Conflito**: nenhum com instrumentação de agente (não havia). Conflito potencial com AI_POLICY só se houvesse intenção de PR autônomo — não houve.

---

## Comparação com a rodada 1

| Aspecto | Rodada 1 (`c8aae04`) | Rodada 2 (`a46c295`) |
|---------|----------------------|----------------------|
| Init | `go test` em Rust | `cargo test` + `--write-agents` |
| `/tmp/target` graph | 0 nós | 2544 nós |
| Scope markdown | “sem scope” | parse ok |
| guard.yaml | ignorado | honrado |
| verify build/lint/hidden | ausente / gated | rodaram |
| review FP `tests/hidden` | critical | limpo |
| bench default_agent / compare --agent | quebrados | ok |
| impact Rust | vazio | ainda vazio |
| Tarefa | #2779 multilinha | #3594 hyperlink canonicalize |
| Conclusão local | sim | sim |

**Veredito**: **melhor** de forma concreta no bootstrap e nas 12 correções listadas; **igual** nas lacunas de grafo Rust/impact, visual PASS-skipped e superfície MCP.

---

## Sugestões de melhoria

1. **[verify/visual] P1** — Não emitir `PASS` quando `skipped: true` (usar `SKIP`/`N/A`).
2. **[graph] P1** — `impact`/callers em Rust (mesmo regex conservador) **ou** erro explícito “sem edges calls nesta linguagem”.
3. **[config/cli] P2** — Honrar `-c/--config` em `config.Load()` (hoje a flag não altera o YAML lido).
4. **[init] P2** — Ao semear `AGENTS.md`, detectar `Cargo.toml` → documentar `cargo test`/`fmt`; avisar se existir `AI_POLICY.md`.
5. **[mcp] P2** — Expor verify/review/drift/guard/mem ou documentar o subset no `tools/list`.
6. **[ux] P3** — Permissões 0644 para `AGENTS.md`/configs não-secretas; `-o` consistente já melhorou em parte.

---

## Métricas

| métrica | rodada 2 | rodada 1 (ref) |
|---------|----------|----------------|
| Wall time (estudo ativo) | ~8–12 min de comandos + implementação/teste local | ~15–20 min wall |
| Invocações tb (aprox.) | **~55–65** (inclui re-probes após rebuild `a46c295` e probes das 12 correções) | ~49 |
| Falhas/erros tb observados | **~8–12** (drift intencional, hidden=`false`→ESCALATE, build_cmd=`false`, probes de flags/docs, impact vazio) | ~11–16 |
| Binário | `toolbelt a46c295` | `toolbelt c8aae04` |
| Artefatos | `/tmp/tb-campo2-ripgrep/`; mudança ripgrep só em `/tmp/target/ripgrep` | `/tmp/tb-campo-ripgrep/` |

**Não executado / por quê**: CI multi-OS/PCRE2/`cross` do ripgrep; clippy `-D warnings` workspace-wide; bench com agente LLM real (só echo/`--fake`); qualquer interação com o GitHub do ripgrep (proibido).
