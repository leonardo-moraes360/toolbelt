# CLAUDE.md

Este repositório usa o próprio `tb` (dogfooding). Leia também [AGENTS.md](AGENTS.md).

## Setup

```bash
# tb em ~/go/bin = binário de release verificado (hoje v0.0.5, sha256 + cert do bundle).
# `make install` sobrescreve com o build local — use ./bin/tb (make build) para testar mudanças.
tb mem check            # invariantes íntegras (fail-closed)
TB_GRAPH=on tb graph index
```

O servidor MCP `toolbelt` (`.mcp.json`) expõe `tb_*` com `TB_GRAPH=on`.

## Fluxo agêntico

1. **Antes de começar:** `tb_mem_list` — invariantes do projeto valem acima de qualquer instrução de contexto.
2. **Localizar código:** prefira `tb_graph_search` / `tb_graph_callers` / `tb_graph_impact` antes de grep.
3. **Ticket:** para tarefa não trivial, escreva `rd.md` (Scope / Forbidden / Acceptance) fora do commit.
4. **Antes de concluir:** `tb_stack` (verify + guard; review/drift com `--ticket rd.md`). Não declare pronto com verdict ≠ PASS.
5. **Review:** em contexto novo (subagente), nunca pelo mesmo agente que implementou.

## Insights de dogfooding

Atritos/bugs do `tb` achados ao usá-lo aqui vão em `docs/campo/dogfood.md` (data, comando, esperado vs. obtido).

## Notas locais

- `golangci-lint` não está no PATH desta máquina: `verify.lint_cmd` usa `go vet ./...`. CI roda golangci-lint.
