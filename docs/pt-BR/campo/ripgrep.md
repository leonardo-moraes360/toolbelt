[🇺🇸 English](../../campo/ripgrep.md) | 🇧🇷 Português

# Campo: BurntSushi/ripgrep

Estudo de campo do `tb` (grupo: sem instrumentação de agente de IA), clone local apenas — sem push/issue/PR/comentário no upstream.

- **tb build**: `make build` → `toolbelt c8aae04` (commit `c8aae04`, built `2026-11-15T20:44:23Z`)
- **Alvo**: clone shallow em `/tmp/target/ripgrep` (HEAD inicial `178cc02`)
- **Janela**: ~2026-11-15T20:44Z → 20:52Z UTC

---

## Repositório

| Campo | Valor |
|-------|--------|
| Nome | [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) |
| Linguagem | Rust (edition 2024, `rust-version` 1.96; workspace de crates `grep-*`) |
| Tamanho | ~104 `.rs` / ~55–56k LOC; clone shallow ~4.8 MiB tree; working tree com `target/` ~699 MiB após build |
| Instrumentação AI existente | **Só** `AI_POLICY.md` (+ menção em `CONTRIBUTING.md`). Sem `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.cursor/`. A política **proíbe agentes autônomos** em contribuições ao projeto; este estudo ficou 100% local. |
| Toolchain | Rustup `stable` atualizado para **1.100.0** (PATH inicial tinha 1.83 e não compilava). `cargo test --tests` e `cargo build` ok. `rustfmt`/`clippy` disponíveis. **Não** rodei a suíte CI completa multi-target/`cross`/PCRE2 matrix; `cargo clippy -D warnings` no grafo de deps falhou por lints pré-existentes em crates (ex. `grep-matcher`), não por causa da mudança. |

---

## Tarefa escolhida

| Campo | Valor |
|-------|--------|
| O quê | Corrigir números de linha (e offsets) no printer padrão quando `-U` + `-r` batem em matches multilinha **adjacentes** (#2779) |
| Por quê | Bug aberto confirmado no master (`15.2.0`/HEAD); maintainer apontou `grep-printer`; escopo pequeno-médio com teste de regressão claro |
| Issue | https://github.com/BurntSushi/ripgrep/issues/2779 |
| Completou? | **Sim, só local** (commit `f2df235` em `/tmp/target/ripgrep`; **não** enviado ao GitHub) |
| Resultado nativo | Antes: `1:fnord / 2:boccob / …`. Depois: `1 / 4 / 7 / 10` e `-b` `0 / 30 / 61 / 98`. `cargo test -q -p grep-printer --lib` → 110 ok; `cargo test -q --tests` → 114 + **333** ok (inclui `replace_adjacent_line_numbers` e `r2095`). `cargo fmt --check` ok após `cargo fmt`. |

Arquivos tocados: `crates/printer/src/util.rs`, `crates/printer/src/standard.rs`, `tests/multiline.rs`.

---

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | Criou `.toolbelt/` + `config.yaml` (mode **600**); defaults de verify = `go test ./...` | atrapalhou | Repo Rust; agente precisa reescrever verify à mão. Sem `AGENTS.md`, sem detecção Cargo. |
| `tb mem init` / `add` / `list` / `check` | `invariants.yaml` mode **600**; `add` aceita regra posicional; `mem check` OK (retencao 1.00) | neutro | `add --type/--title` (docs) → `unknown flag: --type`. Invariantes genéricas (`tests/hidden/`) pouco alinhadas ao ripgrep. |
| `tb graph index --enable` | 2546 nós, 3225 arestas → `.toolbelt/graph.json` (~1.2 MiB) | ajudou | Indexação Rust funcionou; ~0.2 s. |
| `tb graph stats --enable` | Contagens por tipo (functions 1661, etc.) | neutro | Útil para sanidade; pouco para a tarefa. |
| `tb graph search --enable replace` | Achou `Replacer`, `replace_all`, testes, GUIDE | ajudou | Acelerou achar o printer vs grep cego. |
| `tb graph impact --enable --file …` | Sem `--file` → erro; **com** `--file` → `(vazio)` / JSON `null` | atrapalhou | Não apontou callers/dependentes do `standard.rs` apesar do índice. |
| `tb graph deps` / `callers` | Exigem `--file`/`--module` e `--function`; invocação posicional falha | atrapalhou | UX diferente da intuição/`docs` implícita. |
| Ticket + Scope (`rd.md`) | Markdown `## Scope` **não** parseia; precisa `scope: a, b` e `base_commit: …` | atrapalhou | Com formato errado: review → `sem scope declarado`; drift → `out_of_scope: null` mesmo com paths errados. |
| `tb verify run` (cargo) | `PASS` (iter 1); `hidden_available: false` | ajudou (parcial) | Com `--test-cmd 'cargo test -q --tests'` refletiu a suíte real da mudança. |
| `tb verify` default/Go | `ESCALATE` (3 iters) com `go test ./...` | atrapalhou | Default pós-`init` inviável; esconde o problema até configurar. |
| `tb verify --ruler visual` | `verdict: PASS`, **`skipped: true`**, `skip_reason: manifesto de telas ausente` | neutro | Skip gracioso; verdict PASS pode confundir. |
| `tb review` | Com scope correto: `findings: []`. `-o` inválido (só `--output`) | neutro→atrapalhou | FP crítico se o diff só **menciona** a string `tests/hidden` (ver Problemas conhecidos). |
| `tb drift` | Detecta fora de scope com `scope:` correto; globs `*` e `/**` **funcionam** | ajudou | Exit ≠0 + lista `out_of_scope` no caso `crates/core/main.rs` vs diff real. |
| `tb guard check` | Sem violações no fix; viola `protected_path` em `.toolbelt/thresholds.yaml` | ajudou (parcial) | Paths **hardcoded**; `.toolbelt/guard.yaml` criado → **ignorado**. |
| `tb bench run/compare` | `--fake` ok em `bugfix-go`; `compare --agent` inexistente; `default_agent` do config **ignorado** | neutro | Bench usa fixtures do próprio toolbelt, não do ripgrep. |
| `tb mcp` | `tools/list` via stdio: `tb_env`, `tb_run`, `tb_graph_*`, `tb_version`, `tb_echo` | neutro | Subiu; não cobre review/verify/drift/guard/mem. |
| `tb version` | Com `make`: `c8aae04`. `go build` sem ldflags: `0.1.0-dev`, commit/built **`unknown`** | neutro | Confirma gap de build sem Makefile. |
| `tb ticket` | `unknown command` | atrapalhou | Documentado em `docs/cli.md`, ausente no binário. |

---

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare --agent …` → `unknown flag: --agent` |
| `guard.yaml` ignored | **confirmado** | Arquivo com `protected_paths: crates/printer/**`; `guard check` no diff do printer → `violations: null`. Código em `internal/guard` usa slice hardcoded. |
| Outdated CLI docs | **confirmado** | Docs: `tb mem add --type`, `tb ticket`, `guard.yaml` como manifesto; CLI real diverge. |
| version shows `unknown` | **confirmado (parcial)** | Sem ldflags: versão `0.1.0-dev`, `commit: unknown`, `built: unknown`. Com `make build`, ok. |
| config files created 0600 | **confirmado** | `.toolbelt/config.yaml`, `invariants.yaml`, outputs JSON de verify → mode `600`. |
| visual ruler `skipped=false` when skipping | **refutado (neste run)** | JSON: `"skipped": true`, `"skip_reason": "manifesto de telas ausente"`. |
| `default_agent` ignored | **confirmado** | `bench.default_agent: "echo AGENT_SHOULD_RUN {prompt}"` no config; `tb bench run` sem flags → `--agent ou --fake é obrigatório`. |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | Diff em `.toolbelt/thresholds.yaml` → guard `protected_path`; verify `verdict: FAIL`, `iterations: 0`, exit 1. |
| default fake agent needs `--fake-config` | **refutado (uso básico)** | `tb bench run --fake --task bugfix-go` → 100% sucesso sem `--fake-config`. Flag só customiza o comando. |
| `mem add` positional | **confirmado** | Usage: `tb mem add <regra>`; `--type` rejeitado. |
| Drift/Scope need literal paths (no globs) | **refutado (parcial)** | `scope: crates/printer/src/*.rs, tests/**` casou os arquivos. O que **quebra** é formato markdown `## Scope` / listas `- path` (não parseados). |
| `graph impact` needs `--file` | **confirmado** | Sem flag: `--file é obrigatório`. |
| review false positives on tests/hidden invariant | **confirmado** | Commit só com comentário `// … tests/hidden …` → finding `critical` “possível violação… NUNCA modifique arquivos em tests/hidden/”. |
| verify runs only partial checks / misses build errors | **confirmado** | Com `--test-cmd 'cargo test -q -p grep-printer --lib'`, verify `PASS` enquanto `crates/core/main.rs` inválido → `cargo build` exit 101. |
| tb does not push agent to complete whole task | **confirmado (observacional)** | Init não gerou checklist/AGENTS; não detectou Cargo; Scope/docs errados; verify default Go; impact vazio. A tarefa só fechou por iniciativa fora do “happy path” do tb. |

---

## Problemas novos

1. **P1 — `tb init` assume Go em repo Rust**  
   - Repro: `tb init` em ripgrep → `verify.test_cmd: go test ./...`.  
   - Esperado: detectar `Cargo.toml` / perguntar / deixar explícito “não detectado”.  
   - Atual: defaults Go; verify Go → `ESCALATE`.

2. **P1 — formato de ticket/Scope não documentado de forma operacional**  
   - Repro: `## Scope` + lista `- path` → review “sem scope”; drift não marca nada.  
   - Esperado: aceitar heading markdown **ou** docs/CLI mostrarem `scope: a, b` + frontmatter YAML.  
   - Atual: só `scope:` / `escopo:` em linha (regex em `parseTicketMarkdown`).

3. **P2 — `graph impact` retorna vazio em crate Rust indexada**  
   - Repro: após `graph index`, `tb graph impact --enable --file crates/printer/src/standard.rs` → vazio/`null`.  
   - Esperado: edges defines/imports ou mensagem “sem impacto indexado”.  
   - Atual: silêncio; search encontra símbolos no mesmo arquivo.

4. **P2 — `verify` visual: `PASS` + `skipped: true`**  
   - Repro: `--ruler visual --skip-unavailable` sem `screens.yaml`.  
   - Esperado: verdict `SKIP`/`N/A`, não `PASS`.  
   - Atual: fácil interpretar como verificação visual ok.

5. **P2 — flags curtas inconsistentes**  
   - Repro: `tb review -o file` / `tb drift -o file` → `unknown shorthand flag: 'o'`; verify aceita `-o`.  
   - Esperado: `-o` alias de `--output` em todos.  

6. **P3 — MCP tool surface << CLI**  
   - Repro: `tools/list` sem mem/verify/review/drift/guard/bench.  
   - Esperado: paridade ou discover documentado das lacunas.

7. **P3 — JSON de verify criado 0600**  
   - Mesmo padrão dos configs; atrapalha leitura por outros uids no CI compartilhado.

---

## O que o tb pegou e o que deixou passar

**Pegou**
- Index/`graph search` localizou printer/replacer.
- `verify` com `cargo test -q --tests` passou no fix final (e falharia com `go test`).
- `drift`/`review` com `scope:` correto validaram o perímetro.
- `guard` bloqueou alteração de `.toolbelt/thresholds.yaml` (path hardcoded).
- `mem check` intacto; MCP respondeu.

**Deixou passar / não ajudou**
- Não sugeriu a causa raiz nem o formato de ticket.
- `graph impact` não guiou o blast radius.
- Não rodaria build completo se `test_cmd` for só `-p grep-printer` (demo: main quebrado + verify PASS).
- `hidden_available: false` sempre aqui — sem suíte oculta gerada/descoberta.
- Review FP por menção textual a `tests/hidden`.
- Não leu `AI_POLICY.md` / não avisou conflito com contribuição autônoma (só relevante se alguém fosse abrir PR — não abrimos).

**Nativo vs tb**: a regressão `r2095` foi vista por `cargo test --tests`, não por um ruler específico do tb; o tb só espelha o `test_cmd` configurado.

---

## Interação com instrumentação existente / onboarding

- **Existente**: apenas `AI_POLICY.md` (política humana; proíbe agentes autônomos no upstream). Nada do tipo AGENTS/Cursor skills.
- **`tb init` gerou**: `.toolbelt/{config.yaml,bench,logs,vault}`; depois `mem init` → `invariants.yaml`; graph → `graph.json`. **Não** gerou `AGENTS.md`, scripts de verify Rust, nem importou CONTRIBUTING/AI_POLICY.
- **Qualidade**: estrutura ok; conteúdo desconectado (Go, `tests/hidden`, thresholds).
- **Faltou**: detecção de `Cargo.toml` → `cargo test`/`cargo fmt`/`clippy`; scaffold de ticket com `scope:`/`base_commit:` exemplos; opcional `AGENTS.md` mínimo; leitura de políticas do repo; `graph impact` útil em Rust; MCP alinhado ao workflow.

---

## Sugestões de melhoria

Ordenadas; tag por área.

1. **[init/onboarding] P0** — Detectar ecossistema (Cargo/Go/npm) e preencher `verify.test_cmd` / sugerir lint/build; gerar `AGENTS.md` stub + exemplo de `rd.md` com `scope:`/`base_commit:`.
2. **[docs/cli] P0** — Sincronizar `docs/cli.md` com a árvore real (`mem add`, sem `ticket`, flags `compare`, `guard` hardcoded vs `guard.yaml`).
3. **[drift/review] P1** — Aceitar `## Scope` markdown lists; documentar globs; reduzir FP de invariantes (match em paths do diff, não substring no texto).
4. **[guard] P1** — Honrar `.toolbelt/guard.yaml` ou remover das docs; não abortar verify em paths de config do próprio toolbelt sem mandate.
5. **[graph] P1** — Fazer `impact` retornar algo útil em Rust (defines/imports) ou erro explícito.
6. **[verify] P1** — Separar `SKIP` de `PASS`; opção de `build_cmd` além de `test_cmd`; avisar quando `hidden` indisponível.
7. **[bench] P2** — Respeitar `default_agent`; alinhar `compare` com `--agent` ou documentar só configs/`--fake`.
8. **[mcp] P2** — Expor verify/review/drift/mem ou documentar subset.
9. **[ux] P3** — `-o` consistente; permissões 0644 para configs não-secretas.

---

## Métricas

| métrica | valor |
|---------|--------|
| Wall time (estudo ativo) | ~8–10 min de execução cronometrada de comandos (~12.6 s somados só nos `time` do log de tb/curtos) + ~15–20 min wall total da sessão incluindo leitura/implementação |
| Invocações tb (aprox.) | **~49** (init/mem/graph/verify/review/drift/guard/bench/mcp/version + retentativas) |
| Falhas/erros tb observados | **~11–16** exits/erros de UX (flags, defaults, drift intencional, guard/verify em thresholds, compare `--agent`, `ticket`, etc.) — vários foram probes de problemas conhecidos |
| Artefatos | evidências em `/tmp/tb-campo-ripgrep/`; mudança ripgrep só em `/tmp/target/ripgrep` (não no remoto) |

**Não executado / por quê**: CI multi-OS do ripgrep; matrix PCRE2/`cross`; clippy `-D warnings` workspace-wide (lints pré-existentes); bench A/B com agente real (só `--fake`); interação qualquer com o GitHub do ripgrep (proibido pelas regras do campo).
