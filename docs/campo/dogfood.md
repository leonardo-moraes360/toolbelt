🇧🇷 Português | [🇺🇸 English](../en/campo/dogfood.md)

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
