[🇺🇸 English](../../../campo/rodada2/ollama.md) | 🇧🇷 Português

# Estudo de campo rodada 2: ollama/ollama

Estudo em 2026-11-16 com `tb` buildado de `main` (`make build` → `toolbelt a46c295`). Clone shallow local em `/tmp/target/ollama`. Grupo: **com instrumentação AI existente**. **Nenhuma interação** com o upstream (sem push, issue, PR ou comentário).

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | [ollama/ollama](https://github.com/ollama/ollama) |
| Linguagem | Go (~1097 `.go`), UI TS, C/C++ nativo (llama/MLX) |
| Tamanho | ~53 MiB shallow; 1657 arquivos (excl. `.git`); `go 1.26.10` no `go.mod` |
| Instrumentação AI existente | `AGENTS.md` (cmake + `go build`/`go run`); `CLAUDE.md` aponta para `AGENTS.md`; docs de integrações em `docs/integrations/` |
| Toolchain | **Parcial → melhor que a rodada 1.** `GOTOOLCHAIN=auto` → Go 1.26.10; `go test ./format/`, `go vet ./format/` e **`go build .` OK** (~51 s). CMake: `cmake` presente, mas configure falhou (`Ninja` ausente; `CMAKE_C_COMPILER`/`CXX` não setados no probe). `golangci-lint` ausente. GPU backends não tentados. |

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Corrigir `HumanNumber` para não emitir `1000K` quando o arredondamento cruza a fronteira K→M (`999500` → `1M`). Ajuste em `format/format.go` + casos em `format/format_test.go`. |
| Por quê | Bug aberto, puro Go, pacote pequeno com testes; escopo diferente da rodada 1. |
| Issue | https://github.com/ollama/ollama/issues/19490 |
| Completada? | **Sim, localmente** (commit `9ffcee3` só no clone). Não enviada ao upstream. |
| Native test/lint/build | `go test ./format/ -count=1` PASS; `go vet ./format/` PASS; `go build .` PASS. CMake nativo completo **não** configurado (ninja). |
| Rodada 1 (comparação) | Flag `verbose` ausente em `ollama launch` (issue #14654) em `cmd/`. |

## Verificação das 12 correções

| # | correção | funcionou/não funcionou/não testado/não se aplica | evidência |
|---|----------|-----------------------------------------------------|-----------|
| 1 | graph SkipDirs na raiz do walk | **funcionou** | `tb graph --enable index /tmp/target` → `8925 nós, 65764 arestas` (raiz `target` não zera o grafo); rebuild em `/tmp/target/ollama` → `8923` nós |
| 2 | verify `hidden_test_cmd` sem `tests/hidden/` | **funcionou** | Sem dir `tests/hidden/`; `hidden_test_cmd='go test ./format/ -run HumanNumber -count=1'` → JSON `hidden_available: true`, `hidden_passed: true`, `verdict: PASS` |
| 3 | guard `--strict --json` exit ≠ 0 | **funcionou** | Diff em `tests/hidden/x.go`: `--strict --json` → EXIT **1** e JSON com `protected_path`; idem para `.toolbelt/thresholds.yaml` |
| 4 | guard lê `protected_paths` de `guard.yaml` | **funcionou** | `.toolbelt/guard.yaml` com `format/format.go` e `format/*` → violations nesses arquivos no diff do fix |
| 5 | init detecção de toolchain | **funcionou** | `tb init` em repo com `go.mod` → `verify.test_cmd: go test ./...` (não default cego sem detecção) |
| 6 | review `tests/hidden` por path | **funcionou** | Ticket só *menciona* `tests/hidden/**` sem tocar o dir → `findings: []` em `--standards invariants` |
| 7 | bench `default_agent` e `compare --agent` | **funcionou** | `default_agent: echo AGENT_OK {prompt}` + `tb bench run -r 1` **sem** `--agent`/`--fake` → EXIT 0; `bench compare ... --agent 'echo COMPARE {prompt}'` → EXIT 0 |
| 8 | review/drift `## Scope`/`## Escopo`, listas e `**` | **funcionou** | `## Scope` com bullets `` `format/**` `` → drift limpo; `## Escopo` + `format/**` OK; scope `cmd/**` → `out_of_scope: [format/...]` EXIT 1; `format/**/*.go` e `format/*.go` casam |
| 9 | verify `build_cmd` e `lint_cmd` | **funcionou** | Config com `build_cmd=go build ./format/`, `lint_cmd=go vet ./format/` → `build_ran/passed: true`, `lint_ran/passed: true` no verify PASS |
| 10 | init lê `AGENTS.md`/`CLAUDE.md` e `--write-agents` | **funcionou** | Init logou `sources=AGENTS.md,CLAUDE.md` e hints (`cmake`, `go build`, `go run . serve`); gravou `.toolbelt/agent_docs.yaml`; `--write-agents` não sobrescreveu `AGENTS.md` existente |
| 11 | graph Python `calls` | **não se aplica** | Repo Go; não exercitado em `.py`. Em Go, `callers --function HumanNumber` retornou callers reais (`cmd.showInfo`, `server.GetModelInfo`, …) |
| 12 | visual skipped agregado + docs CLI alinhados | **parcial** | Visual: `skipped: true`, `skip_reason: manifesto de telas ausente`. Docs: `tb serve`/`mem add` melhor documentados; ainda há exemplo `tb graph index --path` e menção a `--manifest` no guard enquanto a CLI de `guard check` só expõe `--strict` |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb version` | OK `a46c295` | neutro | build com ldflags |
| `tb init` / `--write-agents` | OK; toolchain Go + `agent_docs.yaml` | **ajudou** | hints do `AGENTS.md`; config `0600` |
| `tb mem init/list/add/check` | OK | ajudou | invariantes anti-cheat + mandate de escopo |
| `tb graph index/rebuild/search/impact/callers` | OK em escala (~9k nós) | **ajudou** | achou `HumanNumber`; impact/callers úteis; index `/tmp/target` OK |
| `tb drift` | OK com `## Scope` | **ajudou** | limpo no mandate; EXIT 1 com scope errado |
| `tb review` | OK + warnings completeness | atrapalhou (parcial) | 3 warnings de aceite apesar de teste `999500→1M` no diff |
| `tb verify run` (tests + build/lint/hidden) | PASS | **ajudou** | `hidden_available: true` sem pasta `tests/hidden/` |
| `tb verify --ruler visual` | PASS skipped | neutro | `skipped: true` |
| `tb guard check` + `guard.yaml` | detecta custom + hardcoded | ajudou / atrapalhou | custom funciona; com yaml protegendo os arquivos da tarefa, **verify FAIL** `iterations: 0` |
| `tb bench run` (default_agent) / `--fake` / `compare --agent` | OK | ajudou | fixtures do toolbelt (`bugfix-go`) |
| `tb mcp` `tools/list` | OK | ajudou | version, echo, env, run, graph_* |
| Checks nativos | test/vet/build Go OK | — | alinhados ao `verify` quando `test_cmd`/`build_cmd`/`lint_cmd` apontam a `./format/` |

## Problemas que continuam

- **Review completeness com FP/ruído**: critérios de aceite marcados como “pouco evidenciados” mesmo com o caso `999500` e `TestHumanNumber` no diff (mesmo padrão da rodada 1).
- **Configs `0600`**: `config.yaml` / `agent_docs.yaml` continuam mode `600`.
- **CMake/ninja ainda fora do VM**: build nativo completo incompleto; `tb` não bloqueia “done” por isso (hints do AGENTS ajudam, mas não viram gate).
- **Docs CLI ainda com resíduos**: exemplo `graph index --path`; `--manifest` descrito para guard sem flag na CLI.
- **Verify aborta cedo em path protegido**: comportamento anti-cheat desejado nos hardcoded; agora também via `guard.yaml` — fácil falhar a verificação se o manifesto listar arquivos da própria tarefa.

## Problemas novos

1. **P2 — `guard.yaml` + verify: proteger o path da tarefa vira FAIL imediato**  
   - Repro: `protected_paths: [format/format.go]`; `tb verify run` no commit que altera esse arquivo → `verdict: FAIL`, `iterations: 0`, sem rodar build/test.  
   - Esperado: manifesto custom distinguir “não trapacear” vs “arquivos sensíveis permitidos no mandate”; ou warn em vez de abortar verify quando o path está no Scope.  
   - Atual: qualquer `protected_path` error aborta verify.

2. **P3 — Exemplo `tb graph index --path` ainda na docs**  
   - Repro: `docs/cli.md` ~L284 mostra `--path`; CLI real usa argumento posicional.  
   - Esperado: exemplos só com flags/args aceitos (correção #12 incompleta).

3. **P3 — Docs citam `--manifest` no guard; CLI não tem a flag**  
   - Repro: `tb guard check --help` só `--strict`; docs ainda falam `--manifest`.  
   - Esperado: docs = CLI (carrega `.toolbelt/guard.yaml` implicitamente).

## O que o tb pegou e o que deixou passar

**Pegou**
- Drift de escopo com `## Scope`/`## Escopo` e globs `**`.
- Guard custom (`format/*`) e hardcoded (`tests/hidden`, `thresholds.yaml`) com `--strict --json` falhando o exit code.
- Verify com `build_cmd`/`lint_cmd`/`hidden_test_cmd` configurados → PASS real no pacote `format`.
- Grafo: search/impact/callers de `HumanNumber` após index em `/tmp/target`.
- Onboarding: leitura de `AGENTS.md`/`CLAUDE.md` + hints de build.

**Deixou passar / enfraqueceu**
- Review completeness ruidoso (não confirma aceite óbvio pelo diff de teste).
- Não transformou hints do AGENTS (cmake/ninja) em `verify.build_cmd` nativo completo — só Go detectado.
- Com `guard.yaml` mal calibrado, o próprio verify impede validar a tarefa (falso “anti-cheat” sobre o mandate).
- Ruler visual sem manifesto → skip (esperado); zero sinal visual.

## Interação com instrumentação existente / onboarding

- **`AGENTS.md` / `CLAUDE.md`**: `tb init` **leu** e registrou em `.toolbelt/agent_docs.yaml` com hints (`cmake -B build .`, `cmake --build`, `go build .`, `go run . serve`). Melhora clara vs rodada 1 (antes: zero leitura).
- **`--write-agents`**: disponível; não sobrescreveu o `AGENTS.md` existente (correto).
- **Duplicação**: invariantes `mem` genéricas vs regras de build do AGENTS ainda paralelas; hints não preencheram automaticamente `build_cmd` cmake.
- **Onboarding**: melhor bootstrap (toolchain Go + agent docs). Ainda sem progress log / ticket gerado; `rd.md` manual.

## Comparação com a rodada 1

| Aspecto | Rodada 1 | Rodada 2 | Veredito |
|---------|----------|----------|----------|
| SkipDirs `/tmp/target` | risco de 0 nós | 8925 nós ao indexar `/tmp/target` | **melhor** |
| `guard.yaml` | ignorado | lido e aplicado | **melhor** |
| `--strict --json` | exit 0 com violação | exit 1 | **melhor** |
| `default_agent` / `compare --agent` | quebrados | funcionam | **melhor** |
| `## Scope` Markdown | frágil / YAML | headings + bullets + `**` OK | **melhor** |
| `hidden_test_cmd` | gated por dir | roda sem dir | **melhor** |
| `build_cmd`/`lint_cmd` | inertes | honrados | **melhor** |
| AGENTS.md no init | ignorado | lido + hints | **melhor** |
| Review completeness | ruído | ruído semelhante | **igual** |
| Build nativo cmake | falhou | ainda incompleto (ninja); `go build .` agora OK | **parcialmente melhor** |
| Tarefa | `cmd` verbose | `format` HumanNumber | diferente (como pedido) |

No geral: **substancialmente melhor** nas 12 correções exercitáveis neste repo; lacunas restantes são review completeness, docs residuais e o footgun guard.yaml↔verify.

## Sugestões de melhoria

1. **[P1][guard/verify]** Não abortar verify por `protected_paths` custom que estão no Scope do ticket (ou severidade warning + flag).  
2. **[P1][review]** Completeness: tratar asserts/casos de teste no diff como evidência de aceite.  
3. **[P2][init/verify]** Mapear hints de `AGENTS.md` → sugerir `build_cmd`/`lint_cmd` (cmake quando detectável).  
4. **[P2][docs]** Remover `graph index --path` e `--manifest` órfão; só documentar o que a CLI aceita.  
5. **[P3][config]** Mode menos restritivo para `agent_docs.yaml` não secreto.

## Métricas

| métrica | rodada 2 | rodada 1 (referência) |
|---------|----------|------------------------|
| Wall time da sessão de campo | ~8–12 min (inclui `go build .` ~51 s + download toolchain) | ~6–10 min |
| Invocações `tb` (blocos de comando úteis) | ~30 (exclui 10 EXIT 127 de erro de harness no log) | 59 |
| Falhas tb intencionais (exit ≠ 0) | 3+ (2× guard `--strict`, 1× drift scope errado; verify FAIL com guard.yaml também exit 1) | 13 |
| Soma aproximada de WALLs tb (sem `go build` nativo) | ~5–6 s de comandos tb + verify/bench ~2 s | ~9,7 s |
| Commits locais no clone ollama | 1 feature (`9ffcee3`) + probes descartados | feature + probes |
| Artefato neste repo | apenas `docs/campo/rodada2/ollama.md` | `docs/campo/ollama.md` |
