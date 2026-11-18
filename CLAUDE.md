# CLAUDE.md

Este repositório usa o próprio `tb` (dogfooding). Leia também [AGENTS.md](AGENTS.md).

## Setup

```bash
# tb em ~/go/bin = binário de release verificado (hoje v0.0.5, sha256 + cert do bundle).
# `make install` sobrescreve com o build local — use ./bin/tb (make build) para testar mudanças.
tb mem check            # invariantes íntegras (fail-closed)
TB_GRAPH=on tb graph index
```

O servidor MCP `toolbelt` (`.mcp.json`) expõe `tb_*` com `TB_GRAPH=on` (36 tools na v0.0.5).

## Mapa

- `cmd/tb` — CLI cobra; `cmd/fake-agent` — agente fake do bench.
- `internal/sandbox` (Landlock, fail-closed), `execenv` (allowlist de env), `guard`, `verify`, `review`, `stack` — verificação adversarial (ADR-004/005/008).
- `internal/mem` — invariantes em `.toolbelt/invariants.yaml` (ADR-010).
- `internal/graph` — índice, `search` (BM25), `callers`/`impact`, `explain`/`path` (ADR-011/014).
- `internal/ticket`, `router`, `merge`, `workspace` — Fase 3 (ADR-009).
- `internal/bus` (store SQLite em `.toolbelt/bus/bus.db`) e `internal/busrun` (`tb bus run`) — barramento (ADR-013; specs `docs/bus-protocolo.md`, `docs/bus-harness.md`). Estado privado fica em `$XDG_STATE_HOME/toolbelt/bus/`, nunca no repo.
- `internal/mcp` — servidor MCP stdio sobre os mesmos pacotes.

## Fluxo agêntico

1. **Antes de começar:** `tb_mem_list` — invariantes do projeto valem acima de qualquer instrução de contexto.
2. **Localizar código:** prefira `tb_graph_search` / `tb_graph_callers` / `tb_graph_impact` antes de grep; `tb_graph_explain` / `tb_graph_path` para vizinhança e ligação entre dois nós.
3. **Ticket:** para tarefa não trivial, escreva `rd.md` (Scope / Forbidden / Acceptance) fora do commit.
4. **Antes de concluir:** `tb_stack` (verify + guard; review/drift com `--ticket rd.md`). Não declare pronto com verdict ≠ PASS.
5. **Review:** em contexto novo (subagente), nunca pelo mesmo agente que implementou.
6. **Docs:** PT-BR (`docs/`, `README.md`) e espelho EN (`docs/en/`, `README.en.md`) no mesmo PR; `TestDocsEnglishMirrorComplete` cobra o par.

## Release

Ver `docs/release.md`: mover `[Não Lançado]` do `CHANGELOG.md` para a versão, tag `v*` (dispara goreleaser + cosign + SBOM),
depois baixar o archive, `sha256sum -c` e conferir o cert do bundle antes de instalar em `~/go/bin`.

## Insights de dogfooding

Atritos/bugs do `tb` achados ao usá-lo aqui vão em `docs/campo/dogfood.md` + `docs/en/campo/dogfood.md` (data, comando, esperado vs. obtido).

## Notas locais

- `golangci-lint` não está no PATH desta máquina: `verify.lint_cmd` usa `go vet ./...` (o `tb init` ainda grava `golangci-lint run`; dogfood #1). CI roda golangci-lint.
- `cosign` e `gh attestation` indisponíveis: verificação de release fica em sha256 + SAN/issuer/commit do cert.
- Em `git worktree`, o `tb init` grava o exclude no gitdir da worktree, que o git ignora (dogfood #17); o `.gitignore` cobre o estado local.
