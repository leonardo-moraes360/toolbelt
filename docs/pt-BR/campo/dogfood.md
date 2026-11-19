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
| 28 | R0 itens 5 a 8 restantes | — | pendentes: calibração do matching (corpus de paráfrases), taxa de convergência, refresh perto do vencimento do token, `safe.directory` com outro uid (exige outro usuário) | info |
| 29 | `egm-spike collect` com o access token real vencido (`expiresAt` 4h no passado, refresh token válido até 2026-12-15) | a cópia no HOME descartável renova, ou falha sem tocar a credencial real | `Failed to authenticate: OAuth session expired and could not be refreshed` em todos os goldfish; o `.credentials.json` real manteve hash e mtime. Copiar `claudeAiOauth` só funciona com o access token fresco: o R4 precisa checar `expiresAt` antes da fase 2 e parar com erro claro (ou exigir API key) em vez de rodar 9 goldfish até a falha de auth. Ainda não se sabe se um refresh falho dentro da cópia invalida o refresh token real | alta |
| 30 | `egm-spike collect` com 4 goldfish em paralelo | cada goldfish isolado | cada `runGoldfish` pegava o próprio subreaper e `Registry`: `waitid: no child processes` e um órfão falso. Só do spike (o R3 já exige um `Registry` compartilhado e um `AcquireSubreaper`); a coleta agora é sequencial | info |
