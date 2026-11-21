[🇺🇸 English](../../campo/dogfood.md) | 🇧🇷 Português

# Dogfood — `tb` no próprio repositório

Log de atritos e bugs encontrados ao usar `tb` para desenvolver o toolbelt.
Formato: data · comando · esperado vs. obtido · severidade.

## 2026-11-17 — bootstrap (`v0.0.2-20-ga76cee4`)

Sequência: `make install` → `tb init --write-agents` → `tb mem init` → `tb mem add` (×6)
→ `TB_GRAPH=on tb graph index` → `tb verify run` → `tb mcp` (smoke `tools/list`).

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 1 | `tb init` | `lint_cmd` só se o binário existir no PATH (como `test_cmd`) | grava `golangci-lint run` sem checar PATH; `verify run` falha com `comando não encontrado` | média |
| 2 | `tb verify run --json` | em FAIL/ESCALATE, indicar qual teste/pacote falhou | só `visible_passed: false` + `ESCALATE` após 3 iterações; sem trecho da saída — precisei rodar `go test` à mão | alta |
| 3 | `tb verify run` | — | 3 iterações (~20s) re-rodando a mesma suíte determinística sem mudança de código entre elas | baixa |
| 4 | `go test ./...` (suite do tb) | passa independente do git do host | `internal/review` falha com `init.defaultBranch=main`: `git checkout master` hardcoded (`review_test.go:1279,1326,1407`) | média |
| 5 | `tb init --write-agents` | — | `agent_docs.yaml` registra `sources: [AGENTS.md]` — o arquivo que o próprio init acabou de semear (auto-referência) | baixa |
| 6 | `tb mem edit --ref` | mensagem de erro coerente com o aceito | erro lista só `docs/adr/…`/`docs/decisions/…`, mas `docs/en/adr/…` é aceito; sem forma de limpar `ref` (precisei `retire` + `add`) | baixa |
| 7 | `tb init` | arquivos versionáveis com modo padrão | `config.yaml`/`invariants.yaml` criados `0600` | baixa |
| 8 | `tb graph index` | — | ok: 3960 nós / 14746 arestas em 0,16s; 2940 nós são `section` de docs (PT + EN duplicados) — avaliar ruído na busca | info |
| 9 | `tb mcp` | — | ok: 22 tools no `tools/list` | info |

## 2026-11-17 — release `v0.0.3` (`2eb65c1`)

Verificação: `sha256sum -c` ok; certificado do bundle sigstore com SAN
`…/release.yml@refs/tags/v0.0.3`, issuer GitHub OIDC, commit `2eb65c1` = `v0.0.3^{}`.
Assinatura/Rekor **não** verificados (sem `cosign`); `gh attestation verify` indisponível no `gh` 2.46.

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 2 | `tb verify run --json` | causa da falha no JSON | **resolvido**: `reason`/`error` trazem a saída do `test_cmd` | — |
| 10 | `tb verify run --json` | trecho com as linhas `--- FAIL` | `reason` truncado pelo início (`[no test files]`, `ok …`); falhas de pacotes posteriores somem no `…` | média |
| 11 | `tb init` + `TestDocsEnglishMirrorComplete` | `--write-agents`/bootstrap não quebra a suíte | criar `docs/campo/*.md` sem par em `docs/en/` quebra `go test` — invariante de docs PT/EN pega pelo teste, não pelo `tb guard` | info |
| 12 | `tb verify run --dry-run` | — | ok: `verdict: SKIPPED` + plano | info |

## 2026-11-17 — Fase 3 (ticket/board/router/watchdog/merge)

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 4 | `go test ./...` | — | **resolvido**: `initGitRepo` fixa `-b master` | — |
| 13 | `tb stack … --ticket rd.md` | analisar mudanças não commitadas (working tree) | só compara `--base..--head` (commits); precisei de commit descartável em worktree detached para rodar o stack antes de commitar | média |
| 14 | `tb stack drift review` | — | ok: pegou `internal/review/review_test.go` fora do Scope declarado no rd.md | info |
| 15 | `tb guard` (via stack) | `t.Skip` condicionado a ambiente (Landlock/git ausente) distinguível de skip de trapaça | 24 avisos `test_skip` para skips legítimos de ambiente; ruído | baixa |
| 16 | `tb sandbox run -- sh -c 'echo > /dev/null'` | escrita em `/dev/null` permitida | `Permission denied` (também na v0.0.3): git/shells quebram no sandbox — corrigido nesta fase | alta |

## 2026-11-18 — release `v0.0.5` + novo bootstrap (`000b955`)

Release: `sha256sum -c` ok; certificado do bundle sigstore com SAN
`…/release.yml@refs/tags/v0.0.5`, issuer GitHub OIDC, commit `000b955` = `v0.0.5^{}`.
Assinatura/Rekor e `gh attestation verify` seguem não verificados (sem `cosign`; `gh` sem `attestation`).

Bootstrap do zero numa worktree (`config.yaml`/`agent_docs.yaml`/`invariants.yaml` removidos):
`tb init --write-agents` → `tb mem init` → invariantes antigas restauradas + 7 novas (bus, grafo, CGO, release)
→ `TB_GRAPH=on tb graph index` → `tb verify run` → `tb mcp` (smoke `tools/list`).

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 1 | `tb init` | `lint_cmd` só se o binário existir no PATH | **persiste** na v0.0.5: grava `golangci-lint run` sem `golangci-lint` no PATH; voltei a `go vet ./...` à mão | média |
| 3 | `tb verify run` | — | **resolvido**: 1 iteração (~15s), `PASS` | — |
| 7 | `tb init` / `tb mem init` | arquivos versionáveis com modo padrão | **persiste**: `config.yaml`, `agent_docs.yaml`, `invariants.yaml` criados `0600` | baixa |
| 17 | `tb init` numa `git worktree` | padrões de estado local ignorados pelo git | grava em `.git/worktrees/<wt>/info/exclude`, que o git não lê (só `$GIT_COMMON_DIR/info/exclude`); aqui o `.gitignore` mascara, mas em repo sem `.gitignore` o estado local aparece no `git status` | média |
| 18 | `tb init` (dicas de AGENTS.md/CLAUDE.md) | só comandos | extraiu como dica um comentário de dentro de fence `bash` (`# make install sobrescreve…`) e `golangci-lint` solto da prosa | baixa |
| 19 | `tb init` | — | ok: config nova já traz a seção `router:` com os padrões da Fase 3 | info |
| 20 | `tb graph index` / `tb graph search "bus harness wake"` | função à frente de seções de docs | 4567 nós / 19328 arestas em 0,17s; 4 seções de `docs/` (PT + EN) antes de `func:busrun.wake` — mesmo ruído do #8 | baixa |
| 21 | `tb mcp` | — | ok: 36 tools no `tools/list` (eram 22) | info |

## 2026-11-18 — spike R0 do `tb egm` (`bench/egm-spike`, `claude` 2.1.326)

Ambiente: kernel 7.0, git 2.53, `claude` binário ELF nativo (sem interpretador) em
`~/.local/share/claude/versions/`. Autenticação: cópia só da chave `claudeAiOauth` de
`~/.claude/.credentials.json` para um HOME descartável `0700`, com `CLAUDE_CONFIG_DIR`
apontando para ele. Flags do goldfish: `-p --output-format json --tools "" --strict-mcp-config
--setting-sources "" --no-session-persistence`, prompt por stdin.

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 22 | `egm-spike isolation` | pacote RO, `TMPDIR`/HOME privados, HOME real, credencial, repositório, `/proc/<ppid>/environ`, `/proc/1/environ`, `/dev/pts` e `/dev/shm` ilegíveis, env mínimo, sem órfãos | ok: 16/16 sondas; canário no environ do pai não vaza (a restrição de ptrace da Landlock bloqueia a leitura) | info |
| 23 | `egm-spike claude` (1ª execução) | goldfish responde | `EAI_AGAIN`: `/etc/resolv.conf` é symlink para `/run/systemd/resolve/stub-resolv.conf`, e `/run` não é liberado. Corrigido com regra só-leitura de arquivo no realpath do `resolv.conf`. Provavelmente afeta também os agentes do router | alta |
| 24 | `egm-spike claude` | sem ferramentas, sem `CLAUDE.md`/memórias, atestação válida | ok: `SEM-FERRAMENTAS` com `num_turns=1` e sem `permission_denials`; `NENHUMA` para instruções prévias; nonce ecoado, 3/3 trechos literais. Custo ~US$ 0,03 por goldfish | info |
| 25 | controle fora do sandbox (mesmo prompt de memória) | o canário discrimina | ok: com o HOME real, a resposta cita `graphify` e `token-reducer` do `CLAUDE.md` global. Custo US$ 3,47 (contexto completo do ambiente) | info |
| 26 | credencial real | não muda por causa do spike | ok: hash e `expiresAt` iguais antes e depois dos goldfish; a cópia não sofreu refresh. O arquivo real só foi reescrito pelo `claude` fora do sandbox (uso normal), sem refresh de token | info |
| 27 | `go test ./internal/sandbox/` | regra de tmp compartilhado vale com workdir fora de `/tmp` | `isSubpath` trata `../x` como subcaminho: `addTmpWriteRule` nunca aplica a regra e `ValidatePath` aceita caminhos fora do workdir. Pré-existente; **corrigido** na main por `fix/sandbox-issubpath` | alta |
| 28 | R0 itens 5 a 8 restantes | — | pendentes: `safe.directory` com outro uid (exige outro usuário). Calibração do matching no #31, convergência no #32, refresh no #29 | info |
| 29 | `egm-spike collect` com o access token real vencido (`expiresAt` 4h no passado, refresh token válido até 2026-12-15) | a cópia no HOME descartável renova, ou falha sem tocar a credencial real | `Failed to authenticate: OAuth session expired and could not be refreshed` em todos os goldfish; o `.credentials.json` real manteve hash e mtime. Copiar `claudeAiOauth` só funciona com o access token fresco: o R4 precisa checar `expiresAt` antes da fase 2 e parar com erro claro (ou exigir API key) em vez de rodar 9 goldfish até a falha de auth. Os refreshes falhos dentro das cópias não invalidaram o refresh token real: o `claude` fora do sandbox renovou depois sem novo login | alta |
| 30 | `egm-spike collect` com 4 goldfish em paralelo | cada goldfish isolado | cada `runGoldfish` pegava o próprio subreaper e `Registry`: `waitid: no child processes` e um órfão falso. Só do spike (o R3 já exige um `Registry` compartilhado e um `AcquireSubreaper`); a coleta agora é sequencial | info |
| 31 | `egm-spike collect` + `egm-spike calibrate` (matching do R0) | R18–R19 com merges errados ≤ 5% e matches perdidos ≤ 20% | corpus de 87 pares de paráfrase e 351 distintos (mesmo heading, linhas a ≤ 3 de distância) a partir de 96 findings de 12 goldfish reais sobre `egm-trust.md` (PT) e `vault.md` (EN), em `internal/egm/testdata/paraphrases.json`; custo US$ 1,79. Os padrões da spec (Jaccard, `sim` 0,30, `overlap` 0,5) erram 0,3% dos merges, mas **perdem 53%** das paráfrases. Nenhum ponto com Jaccard atinge as duas metas. Com radicais de 4 caracteres (`sim` 0,23) ou containment (`sim` 0,31) as metas valem no corpus inteiro, mas calibrando num documento e testando no outro perdem 22–24% e erram 4–7% dos merges. O limiar de `overlap` não acrescenta nada (ótimo em 0): heading e linhas próximas já dão a região. Matching só léxico não cumpre o R0; R18–R20 precisam de outro sinal | alta |
| 32 | `egm-spike converge 6 16` (modo removido depois do #33; no commit `637683a`) sobre `egm-trust.md` (PT) e `vault.md` (EN): os 9 revisores da R10 (lentes do spike, sem fontes de código), PRIOR reavaliado pelo dono, um goldfish autor corrigindo as linhagens blocker/major abertas, 2 rodadas limpas sobre o mesmo conteúdo | mediana ≤ 4 rodadas até convergir | nenhum convergiu; o orçamento de US$ 16 acabou (US$ 17,11 gastos, cerca de US$ 2,5 por rodada de 9 revisores mais o autor, não US$ 1,5). Linhagens bloqueantes abertas por rodada: `vault.md` 18 → 6 → 5 → 3 (rodada 5 não rodou); `egm-trust.md` 21 → 24 → 21 → 13 (rodada 4 com 6 de 9 revisores). Nenhuma rodada limpa. Os donos nunca mantiveram uma linhagem do PRIOR aberta (`still_holds: true` 0 vezes em 188 reavaliações), então as bloqueantes são novas a cada rodada: cada edição do autor atrai majors novos. A mediana passa de 4, então pelo R0 `max_rounds` e a regra de rodada limpa precisam ser revistos. Dados em `internal/egm/testdata/paraphrases.json` (`convergence`) | alta |
| 33 | `egm-spike converge2 8 20` (R18–R27 revistos: `same_as` sobre `PRIOR`/`KNOWN`, reserva com radical 4, `late`, correção exige mudança) sobre os mesmos dois documentos | converge dentro de `max_rounds: 8` | nenhum convergiu; o orçamento de US$ 20 acabou (US$ 20,76). Linhagens bloqueantes novas por rodada: `vault.md` 24 → 5 → 1 → 0 → 0, e depois travado numa linhagem reaberta por match léxico na rodada 4, porque o goldfish autor devolveu o documento sem mudança e o `fixed_without_edit` (corretamente) ignorou a correção posterior do dono; com o R26 um `loop next` real pararia esperando uma edição. `egm-trust.md` 15 → 17 → 10 → 7 → 2 (rodada 5 com 6 de 9 revisores); o autor o fez crescer de 73 para 193 linhas. **O `late` quase não teve efeito:** 41 de 42 blockers/majors novos sem vínculo nas rodadas ≥ 2 tocavam linhas alteradas desde a rodada anterior, então os problemas novos vêm das edições, não de ruído dos revisores em outras partes; só 1 linhagem `late`. **O `same_as` nunca foi usado** (0 vínculos); a reserva com radical 4 vinculou 13 findings. Os donos continuaram sem manter linhagem (`still_holds: true` 0 de 79); `fixed_elsewhere` 1. Dados em `paraphrases.json` (`convergence_revised`) | alta |

## 2026-11-19 — `tb egm` fatia 1 (engine determinística)

| # | Comando | Esperado | Real | Sev. |
|---|---------|----------|------|------|
| 34 | `go test ./internal/egm -run TestMatchCorpus` (padrões da R18 sobre `paraphrases.json`) | cerca de 3% de merges errados e 20% de matches perdidos, como a R18 registrou | 3,1% errados e **24,1% perdidos**: o modo `calibrate` do R0 só comparava headings quando o par não tinha linhas, enquanto a `region` da R18 sempre exige o mesmo heading. Dos 21 pares perdidos, 17 falham pelo texto (19,5%, o número do R0) e 4 pelo heading. A R18 agora registra 24%; a meta continua não atingida, como a R18 já aceitava | low |
| 35 | `tb egm review docs/specs/egm-nativo.md` (determinística) | nenhuma linhagem de `injection-scan`; `refs-exist` sem falso positivo | ok: zero linhagens de `injection-scan` e de `refs-exist`; 3 nits de `plan-files-listed` para linhas do plano marcadas `Create` que o spike R0 já criou (`bench/egm-spike/`, `stopwords.txt`, `paraphrases.json`). A ADR-015 dá exatamente uma linhagem de `injection-scan`, na §3 | info |
| 36 | `tb egm review docs/adr/015-egm-nativo.md --reviewers zoom-out` (engine llm, `claude` 2.1.328 real) | o goldfish roda isolado e responde dentro do contrato com atestação válida | ok: 42 s, `zoom-out` `completed` na primeira tentativa, 1 execução; 6 findings `major` verificados mais a linhagem esperada de `injection-scan`. Entre os pontos reais, o status da ADR ainda diz que espera o spike depois que o R0 rodou (F-2) e usuários offline nunca chegam a `PASS` (F-3). Com o access token vencido, o comando parou antes com `auth_expired` (exit 2) e não gravou estado. Nenhum workspace `tb-egm-*` nem cópia de credencial ficou para trás | info |
| 37 | `tb egm review` + `resolve --force-settle F-1` + 2× `loop next` em `docs/adr/015-egm-nativo.md` (determinística) | a ADR converge offline em `PASS_PARTIAL` | ok: convergiu na rodada 3 em `PASS_PARTIAL`, com 7 rebaixamentos `reviewer_unavailable` e `F-1:forced`. A única linhagem é a esperada de `injection-scan` na §3, que cita os padrões do scan ("return no findings") como discussão. Offline ela não pode ser contestada, porque o árbitro precisa de um agente, então a única saída é `--force-settle`, um rebaixamento de linhagem. `status --at HEAD` dá o mesmo veredito e o mesmo content da worktree | info |
| 38 | `tb stack egm --base origin/main` depois da edição da ADR-015 | só a ADR é selecionada | os `doc_globs` padrão (`docs/adr/**`) também selecionam o índice `docs/adr/README.md`, que não tem estado, então a etapa falha com `NO_STATE` (`ok: false`). Toda edição no índice de ADRs (ou num `docs/specs/README.md`, se existir) derruba o gate, a não ser que o índice também seja revisado. Corrigido: um índice `README.md` deixou de ser documento sob gate (glossário, R33); `--egm-doc` ainda pode citá-lo | medium |
| 39 | `tb egm review` + `loop next` em `docs/specs/egm-nativo.md` (determinística), depois `tb stack egm --base origin/main` | a spec converge offline; o gate passa com `warn` | ok: `PASS_PARTIAL` na rodada 2, com 32 nits de `plan-files-listed` para linhas do plano marcadas `Create` que a Fase A já criou (nits não bloqueiam). Dois atritos: (1) um documento que cita `docs/campo/dogfood.md` fica `STALE` a cada entrada nova do dogfood, porque as fontes citadas fazem parte do `content`, então registrar uma rodada a invalida; (2) um `loop next` depois da convergência sobre o mesmo conteúdo abre um ciclo e uma rodada novos em vez de parar, o que com a engine `llm` gastaria cerca de 9 execuções de agente à toa. **Corrigido (2):** o `loop next` agora recusa com `already converged over this content in cycle N` e exit 0, sem rodada nem ciclo novos (R26) | low |

## 2026-11-20 — release `v0.0.7` + rodada 3 da spec do vault

| # | Comando | Esperado | Real | Sev. |
|---|---------|----------|--------|------|
| 40 | `git push origin v0.0.7` (workflow de release) | o goreleaser publica os 6 alvos, já que o CI estava verde | `windows/amd64` e `windows/arm64` falharam: `internal/egm/receipt.go` usava `syscall.O_NOFOLLOW`, que o Windows não tem. O CI só compilava `linux/amd64`. Corrigido com arquivos por build tag (#125) e um passo de cross-build no CI (#127); a tag foi recriada, já que nenhuma release tinha sido publicada | major |
| 41 | `tb egm review docs/specs/vault.md --engine deterministic` | só problemas reais | 2 findings reais de `required-sections` (`## Non-goals` e `## Technical plan` não batem com a caixa do padrão) e 6 reais de `plan-files-listed` (arquivos de teste escritos sem o diretório). Mas 6 majors de `refs-exist` apontam para caminhos que a spec planeja criar ou que são do Obsidian (`.obsidian`, `.trash`, `internal/vault/testdata/obsidian`), e uma spec antes da implementação não converge sem recusá-los um a um. Além disso, o `plan-files-listed` só lê a primeira tabela depois do heading do plano: dividir o plano em uma tabela por fatia transformou 16 arquivos `Create` em findings de `refs-exist` | minor |
| 42 | `tb bus discover` (2026-11-20, build local) | o `.mcp.json` deste repositório aponta para um `tb` conhecido | a entrada `toolbelt` roda um `tb` puro, então `same_tb` é `null`: o cliente o resolve no próprio `PATH`, que aqui é a release em `~/go/bin` (v0.0.8), não o build local em teste. É intencional (o CLAUDE.md manda testar com `./bin/tb`), mas o discover deixa a ambiguidade visível | info |

## 2026-11-21 — índice vetorial V1 (`tb graph index --vectors`, build local `embednative`, all-MiniLM-L6-v2 Q8_0)

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 43 | `tb graph index --vectors` neste repositório | gera o `graph.vec` em poucos minutos | o primeiro lote cheio falhou com `embed_protocol: reply line over 196608 bytes`: o loop de leitura do pai pegava o teto de 64 KiB do handshake antes de o handshake aumentá-lo. Corrigido antes do merge (teste de regressão com vetores de 384 dimensões). Depois da correção a execução foi interrompida em ~17 min: 898 arquivos dão cerca de 11 mil chunks, 70% do `third_party/` (llama.cpp vendorizado), e o build x86-64-v2 embeda um chunk de 1,5 KB em cerca de 135 ms com 4 threads (sem AVX/FMA pelo piso do ADR-018; um texto por chamada, o shim ainda não agrupa sequências), então a execução completa leva cerca de 25 min | medium |
| 44 | `tb graph index --vectors` + `search --mode semantic/hybrid` numa cópia de 24 arquivos (`internal/embed`, `secrets`, `sandbox`, 2 docs) | perguntas por intenção acham o código; reexecuções são incrementais | ok: 125 chunks em 12 s, arquivo no estilo `.env` negado, store 0600. Uma segunda execução embedou 0 e reaproveitou 125; editar um arquivo reembedou 1 chunk. "where are symlinked model files refused" põe `internal/embed/model.go` em primeiro no semântico e no híbrido (o lexical também acha pela palavra "model"); "block network sockets in the child" põe `internal/sandbox/network.go` em primeiro no híbrido. Mudar `query_prefix` dá `vectors_stale` e deixa o `graph.vec` byte a byte igual | info |

## 2026-11-21 — índice vetorial V2 (MCP + avaliação de recall, build `embednative` local)

| # | Comando | Esperado | Obtido | Sev. |
|---|---------|----------|--------|------|
| 45 | `TestSemanticRecall` (`internal/graph/eval/semantic.yaml`, 44 consultas por intenção) | recall@10 do hybrid ≥ lexical + 0,10 | ok: all-MiniLM-L6-v2 Q8_0 (sha256 `263215c3cadd6e16740741a7624ab4cbb6c8e777688bd5331ecfbf5681c2f8ed`), llama.cpp b12600 (`29b52d398b29`), 384 dimensões, build x86-64 local, 8 threads: 417 arquivos e 2701 chunks embedados em 3m03s (`third_party/` e `docs/pt-BR/` pulados). recall@10 lexical 0,273, semantic 0,670, hybrid 0,568 (+0,295), então o recurso deixa o status experimental. O semantic teve a maior nota nesta suíte; o hybrid mantém os acertos por nome exato. Ressalva: a suíte foi escrita pelo ferramental da própria sessão de implementação depois de ler o código (frases de intenção, tokens de nomes excluídos por um teste); consultas escritas de forma independente seriam uma evidência mais forte | info |
| 46 | 3 chamadas `tb_graph_search` com `mode: hybrid` via `tb mcp` com o modelo real | o filho aquecido é reaproveitado | ok: cerca de 10 ms por consulta no filho aquecido, um único processo filho para as três | info |
