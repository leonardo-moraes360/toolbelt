🇧🇷 Português | [🇺🇸 English](../en/campo/gin.md)

# Estudo de campo: gin-gonic/gin

Estudo do grupo **sem instrumentação de agente de IA**, usando `tb` (toolbelt @ `c8aae04`) de ponta a ponta num clone local de [gin-gonic/gin](https://github.com/gin-gonic/gin). Nenhum push, issue, PR ou comentário foi feito no repositório alvo.

## Repositório

| Campo | Valor |
|-------|-------|
| Nome | `gin-gonic/gin` |
| Linguagem | Go (`go 1.26.0` no `go.mod`) |
| Tamanho | shallow clone ~1,7 MiB; **99** arquivos `.go`; ~**24 574** LOC Go |
| Commit base do clone | `0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646` (master raso) |
| Instrumentação AI existente | **Nenhuma** — sem `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.copilot*`, `.aider*`, `GEMINI.md` |
| Toolchain | **Parcialmente ok** |

Toolchain detalhada:

- `go test ./...` — **ok** (~31 s na 1ª corrida com download de deps; ~2 s depois).
- `go build ./...` — **ok**.
- `golangci-lint`: v1.64.8 **falhou** (`Go language version (go1.24) ... lower than ... 1.26.0`). v2.15.0 (built with go1.27) **rodou**; reportou 6 issues pré-existentes no tree (gofumpt/gosec/govet), não introduzidas pela tarefa.
- Browser headless (`google-chrome`) presente, mas captura visual falhou no ambiente (dbus/socket; processo killed) — ruler visual só exercitado em modo skip.

Não configurado / não usado: suite `tests/hidden/` (não existe no gin); App Engine tags especiais além do que os testes padrão cobrem.

## Tarefa escolhida

| Campo | Valor |
|-------|-------|
| O quê | Incluir o **nome do campo** (tag `form`/`uri`) nas mensagens de erro de `BindUnmarshaler` / `encoding.TextUnmarshaler` |
| Por quê | Enhancement realista, escopo pequeno, com teste falhando documentado; maintainer plausivelmente aceitaria |
| Issue | https://github.com/gin-gonic/gin/issues/4296 (há PR alheio #4435 — **não interagimos**; implementação independente só no clone local) |
| Completou? | **Sim**, no clone local |
| Diff principal | `binding/form_mapping.go` (`wrapFieldError` + assinaturas de `trySetCustom`/`trySetUsingParser`); teste `TestMappingCustomUnmarshalParamErrorIncludesFieldName` em `binding/form_mapping_test.go` |
| Checks nativos | `go test ./binding/...` PASS; `go test ./...` PASS; `go build ./...` PASS; `golangci-lint run ./binding/...` FAIL por issues **pré-existentes** (não no arquivo alterado de forma nova relevante) |

## Uso do tb por comando

| comando | resultado | ajudou/atrapalhou/neutro | evidência |
|---------|-----------|--------------------------|-----------|
| `tb init` | EXIT 0; criou `.toolbelt/config.yaml` (mode **600**); defaults `verify.test_cmd=go test ./...`, `hidden_test_cmd=go test ./tests/hidden/...` | neutro | ~5 ms; não detectou gin além do default Go; não gerou `AGENTS.md` nem lint/build |
| `tb mem init/add/list/check` | EXIT 0; 3 invariantes default + 1 adicionada; `mem add` é posicional (`<regra>`), junta args com espaço | ajudou (leve) | `mem check` sha256 e `compactacao_segura: true`; mode 600 em `invariants.yaml` |
| `tb graph` (sem `--enable`/`TB_GRAPH`) | EXIT 1: “grafo desabilitado” | neutro | ADR-011 ok, mas UX exige flag a cada chamada |
| `tb graph --enable index/rebuild/stats` | EXIT 0; ~726→731 nós, ~2572→2581 arestas | ajudou | index ~44–48 ms no gin |
| `tb graph search BindUnmarshaler` | EXIT 0, **(vazio)** mesmo após rebuild | atrapalhou | interface/type name não indexado/encontrável |
| `tb graph search wrapFieldError` | encontra função após rebuild | ajudou | precisa rebuild após mudanças |
| `tb graph callers` posicional | EXIT 1: `--function é obrigatório` | atrapalhou | inconsistente com `search` (posicional) |
| `tb graph deps` posicional | EXIT 1: `--module ou --file é obrigatório` | atrapalhou | idem |
| `tb graph impact` sem `--file` | EXIT 1: `--file é obrigatório` | atrapalhou | confirma problema conhecido |
| `tb graph impact --file binding/form_mapping.go` | EXIT 0; lista callers/impactados úteis | ajudou | setByForm, MapFormWithTag, Bind, etc. |
| ticket/`rd.md` + Scope | criado manualmente (não há `tb ticket`) | neutro | frontmatter YAML + markdown; base SHA do clone |
| `tb verify run` (testes) | PASS com `--test-cmd go test ./binding/...` | ajudou (parcial) | não roda lint/build; `lint_cmd`/`build_cmd` no YAML **ignorados** (só existem `test_cmd`/`hidden_test_cmd` no schema) |
| `tb verify` + hidden | PASS; `Oculto: false (disponível: false)` sem `tests/hidden/` | neutro | default aponta para path inexistente no gin |
| `tb verify --ruler visual` | PASS com screens skipped; top-level `skipped: false` quando refs/captura falham | atrapalhou | ver problemas conhecidos/novos |
| `tb review` | PASS no diff da fix; **FP critical** se o diff só *menciona* `tests/hidden` num comentário | atrapalhou (FP) | agreement 3/3 goldfish no FP |
| `tb drift` | EXIT 0 no scope literal/`binding/`/`binding/*.go`; EXIT 1 em scope errado e em `binding/**/*.go` | ajudou | `**` no meio do glob não funciona como esperado |
| `tb guard check` | limpo no diff da fix; viola `protected_path` em `.toolbelt/thresholds.yaml` | ajudou | paths hardcoded; `guard.yaml` do repo **ignorado** |
| `tb bench run` sem `--agent`/`--fake` | EXIT 1 apesar de `bench.default_agent` no config | atrapalhou | `default_agent` ignorado |
| `tb bench run --fake` | roda sem `--fake-config`, mas **não aplica soluções** → pass 0% | atrapalhou | precisa fake-config com `files_to_modify` para ser útil |
| `tb bench compare` | A/B com `--fake` ok; **não aceita `--agent`** | atrapalhou | flag desconhecida |
| `tb mcp` | sobe em stdio; `initialize`+`tools/list` ok; com `TB_GRAPH=on` expõe graph tools; **sem** mem/verify/review/guard/drift | neutro/parcial | `tb_mem_list` → “ferramenta não encontrada” |
| `tb version` (make build) | mostra commit `c8aae04` | neutro | |
| `tb version` (`go build` sem ldflags) | `0.1.0-dev`, commit/built **unknown** | atrapalhou | |

## Problemas conhecidos

| problema | confirmado/refutado/não testado | evidência |
|----------|--------------------------------|-----------|
| `bench compare` lacks `--agent` | **confirmado** | `tb bench compare ... --agent "echo hi"` → unknown flag; help só tem `--fake` |
| `guard.yaml` ignored | **confirmado** | `.toolbelt/guard.yaml` com `binding/form_mapping.go` protegido; `guard check` no diff da fix → `violations: null`; docs/cli.md ainda documentam o manifesto |
| outdated CLI docs | **confirmado** | `docs/cli.md` descreve `run/proc/ticket/board/md/agent/watchdog/merge`; `tb --help` **não** os lista. README “Status do Projeto” ainda diz **Fase 0** enquanto roadmap marca Fase 2 |
| version shows `unknown` | **confirmado** (build sem ldflags) | `/tmp/tb-nover/tb version` → commit/built `unknown`; `make build` preenche |
| config files created 0600 | **confirmado** | `stat` → `600 .toolbelt/config.yaml`, `600 .toolbelt/invariants.yaml` |
| visual ruler reports `skipped=false` when skipping | **confirmado** | com manifesto+browser, screen `skipped:true` (“referência ausente” / “falha na captura”), top-level `"skipped": false`, `verdict: PASS` |
| `default_agent` ignored | **confirmado** | config com `default_agent: echo-agent-should-be-used`; `tb bench run` → “`--agent` ou `--fake` é obrigatório” |
| guard aborts verify when diff touches hardcoded config path | **confirmado** | tocar `.toolbelt/thresholds.yaml` → guard `protected_path` error; verify `verdict: FAIL`, `iterations: 0` |
| default fake agent needs `--fake-config` | **confirmado** (para ser útil) | `--fake` sozinho completa “agent” mas verification falha (0% pass) sem `files_to_*` |
| `mem add` positional | **confirmado** | `Use: tb mem add <regra>`; `MinimumNArgs(1)` + `strings.Join(args," ")` |
| Drift/Scope need literal paths (no globs) | **parcialmente refutado** | `binding/*.go` e `binding/` funcionam; `binding/**/*.go` **falha** (Match não trata `**` no meio) |
| `graph impact` needs `--file` | **confirmado** | posicional rejeitado; `--file` obrigatório |
| review false positives on tests/hidden invariant | **confirmado** | comentário contendo substring `tests/hidden` → finding critical “possível violação de invariante” (agreement 3) |
| verify runs only partial checks / misses build errors | **confirmado** | `gin.go` com syntax error: `go build ./...` FAIL, `go test ./binding/...` PASS → verify PASS; schema sem lint/build |
| tb does not push agent to complete whole task | **não testado** (sem A/B controlado neste repo) | observação qualitativa: init não cria progress log / checklist de features; não há `tb ticket`/`board`; o fluxo depende do agente auto-organizar |

## Problemas novos

1. **P1 — `graph search` não encontra nomes de interface/tipo**  
   - Repro: `tb graph --enable rebuild && tb graph --enable search BindUnmarshaler` → `(vazio)` enquanto `wrapFieldError`/`mappingByPtr` aparecem.  
   - Esperado: achar a interface em `binding/form_mapping.go`.  
   - Atual: vazio.

2. **P1 — MCP não expõe mem/verify/review/guard/drift**  
   - Repro: `tools/list` (com `TB_GRAPH=on`) → só `tb_version/echo/env/run` + graph_*; `tools/call tb_mem_list` → “ferramenta não encontrada”.  
   - Esperado: parity com CLI Fase 1/2.  
   - Atual: subset mínimo.

3. **P2 — Flags posicionais inconsistentes no graph**  
   - Repro: `tb graph --enable callers trySetCustom` / `deps binding/form_mapping.go` falham; `search` aceita posicional.  
   - Esperado: mesma ergonomia.  
   - Atual: callers/deps/impact só via flags.

4. **P2 — `lint_cmd`/`build_cmd` no YAML são silenciosamente ignorados**  
   - Repro: gravar em `.toolbelt/config.yaml`; `VerifyConfig` só tem `test_cmd`/`hidden_test_cmd`.  
   - Esperado: erro de config desconhecida ou execução.  
   - Atual: ignore silencioso.

5. **P2 — `tb review` sem shorthand `-o` (verify tem `-o`)**  
   - Repro: `tb review ... -o /tmp/x.json` → unknown shorthand; só `--output`.  
   - Esperado: flags de output alinhadas.  
   - Atual: inconsistência UX.

6. **P3 — Glob `**` documentável/inesperado**  
   - Repro: scope `binding/**/*.go` marca out_of_scope; `binding/*.go` ok.  
   - Esperado: ou suporte a `**`, ou docs claras “só `*` e prefixo `dir/` / `dir/**`”.  

7. **P3 — golangci / Go 1.26 no onboarding**  
   - Repro: linter antigo incompatível com `go 1.26.0` do gin.  
   - Esperado: `tb init` avisar toolchain ou sugerir comando de lint detectado.  
   - Atual: nenhum hint.

## O que o tb pegou e o que deixou passar

**Pegou**

- Drift de escopo quando arquivos saem do Scope / batem em `forbidden` (`tests/hidden/`).
- Guard em path hardcoded `.toolbelt/thresholds.yaml` e abort do verify.
- Impacto de `binding/form_mapping.go` via grafo (após `--enable` + `--file`).
- Review limpo no diff legítimo da fix (scope+base ok).
- Invariantes persistidas fora de compactação (`mem check`).

**Deixou passar / falhou**

- Erro de **build** no pacote raiz enquanto `test_cmd` é só `./binding/...` (verify PASS).
- Lint pré-existente / qualquer lint (verify não executa).
- Menção textual a `tests/hidden` → **falso positivo** no review.
- `guard.yaml` custom do projeto.
- Nome da interface `BindUnmarshaler` na busca do grafo.
- Captura visual real (skip gracioso com sinal enganoso no campo top-level `skipped`).

Comparativo nativo: `go test ./...` e `go build ./...` validaram a fix; o tb verify configurado de forma “realista por pacote” mascararia regressão de compilação fora desse pacote.

## Interação com instrumentação existente / onboarding

Gin **não tem** instrumentação de agente. Avaliação do bootstrap `tb`:

| Artefato | Gerado por `tb init` / `mem init`? | Qualidade |
|----------|------------------------------------|-----------|
| `.toolbelt/config.yaml` | sim | útil como esqueleto; defaults Go genéricos; graph languages ligadas; **sem** detecção de `golangci-lint`/Makefile/CI do gin |
| `.toolbelt/invariants.yaml` | via `mem init` | defaults anti-cheat genéricos (`tests/hidden/`, `thresholds.yaml`) pouco alinhados a um lib HTTP sem suite oculta |
| `AGENTS.md` / skills / progress log / features JSON | **não** | lacuna vs achados (`tb init` estilo harness) |
| Detecção automática test/lint/build | **não** | `test_cmd` default `go test ./...` happenstance útil; lint/build não |
| Grafo | sob demanda | rápido no gin; exige `--enable`; search incompleto para types |
| MCP | disponível | não substitui o CLI completo; graph tools só com `TB_GRAPH=on` |

Conclusão: o onboarding **não** deixa um agente “pronto” num repo sem instrumentação — falta briefing do projeto, comandos reais de CI do gin (workflow `.github/workflows/gin.yml`), e superfície MCP incompleta.

## Sugestões de melhoria

Ordenadas por impacto; tag de área do tb:

1. **[verify] P0** — Executar (ou exigir) `build_cmd`/`lint_cmd` além de testes; falhar se o módulo não compila. Rejeitar ou avisar chaves YAML desconhecidas.
2. **[review] P0** — Invariante `tests/hidden`: matching por path alterado, não substring no diff/comentário.
3. **[guard] P1** — Honrar `.toolbelt/guard.yaml` (como a docs prometem) em vez de só lista hardcoded.
4. **[bench] P1** — `bench compare` aceitar `--agent` (paridade com `run`); honrar `default_agent`; documentar que `--fake` sem `--fake-config` não resolve fixtures.
5. **[graph] P1** — Indexar/buscar types & interfaces; aceitar posicionais em callers/deps/impact **ou** documentar só-flags; auto-hint `--enable`.
6. **[init/onboarding] P1** — Detectar comandos de test/lint/build (Makefile, CI, `go.mod`); opcionalmente gerar `AGENTS.md` + progress checklist.
7. **[verify/visual] P1** — Se todas as screens estão `skipped`, top-level `skipped=true` (e/ou verdict distinto de PASS “cheio”).
8. **[mcp] P2** — Expor mem/verify/review/guard/drift; parity CLI.
9. **[cli/docs] P2** — Sincronizar `docs/cli.md` e README “Fase 0” com comandos reais; alinhar `-o`/`--output`.
10. **[version] P3** — Embed de versão via `runtime/debug.BuildInfo` quando ldflags ausentes.
11. **[config] P3** — Revisar mode 0600 vs 0644 para configs não secretas em monorepos compartilhados.

## Métricas

| métrica | valor |
|---------|-------|
| Wall time (timer estudo ativo até fim das evidências CLI) | **~5,2 min** (`2026-11-15T20:45:21Z` → `20:50:17Z`); sessão total do agente (docs+clone+implementação+MCP+redação) maior |
| Invocações `tb` registradas (linhas `CMD:` + MCP/visual/review extras) | **~50** (35 com `CMD:` explícito no log + corridas posteriores de visual/review/MCP/graph) |
| Falhas `tb` (EXIT ≠ 0) no log estruturado | **11** (maioria probes intencionais: graph off, flags faltando, drift, default_agent, compare --agent, verify abort por guard) |
| Crashes do binário | **0** observados |
| Build `tb` usado | `make build` → `toolbelt c8aae04` |

Arquivos de evidência bruta neste VM (não commitados): `/tmp/campo-gin-evidence.log`, `/tmp/verify-*.json`, `/tmp/review-*.json`, clone em `/tmp/target/gin` (commits locais apenas).
