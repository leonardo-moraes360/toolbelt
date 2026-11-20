[🇺🇸 English](../../adr/016-vault-fase-a.md) | 🇧🇷 Português

# ADR-016: Vault Markdown, Fase A — o vault é o único armazenamento

## Status

proposed (2026-11-20). Substitui em parte a [ADR-006](006-inspiracoes-graphify-obsidian.md) (itens 1 e 2 da decisão, "TUI própria") e a [ADR-003](003-sqlite-fts5-grafo.md) ("Sincronização vault → SQLite") na Fase A. Spec: [`docs/specs/vault.md`](../specs/vault.md).

## Contexto

A ADR-006 decidiu que a memória de longo prazo do toolbelt seria um vault Markdown compatível com o Obsidian em `.tb/vault/`, com SQLite + FTS5 como índice derivado alimentado por um watcher (ADR-003) e uma TUI `tb md`. Nada disso foi construído. Desde então:

- o grafo foi para `.toolbelt/graph.json` sob demanda (ADR-011, ADR-014), sem SQLite no `go.mod`;
- as invariantes ficam em `.toolbelt/invariants.yaml`, com um `tb mem check` fail-closed (ADR-010);
- o `tb init` cria `.toolbelt/vault/`, mas o exclui do git como cache local, então nada escrito ali é compartilhado.

A documentação ainda descreve o vault, o SQLite, o `tb md` e a sincronização como existentes.

## Decisão

Na Fase A do vault:

1. **Os arquivos Markdown são o único armazenamento.** Todo comando `tb vault` lê o vault do disco. Não há índice derivado persistente, então não há sincronização, watcher nem regra de conflito "o vault vence" para manter. Um cache por fingerprint (como o `graph.json`) vem antes do SQLite, se o NFR de desempenho da spec falhar.
2. **Local `.toolbelt/vault/`, versionado.** O vault sai do exclude de cache local do git; só o `.obsidian/` dentro dele continua excluído.
3. **As invariantes ficam no `invariants.yaml`.** O vault guarda conhecimento que não é invariante (fatos, decisões, contexto, registros) e nunca substitui o `tb mem`.
4. **Wikilinks ambíguos são erro.** Onde o Obsidian desempata pelo caminho mais curto, o tb reporta `ambiguous_link`.
5. **Sem TUI.** `tb md` e `tb md --obsidian` saem da Fase A; qualquer editor, inclusive o Obsidian, edita o vault.
6. **Integração com o grafo só com `TB_GRAPH=on`** (ADR-011): notas, wikilinks e tags viram nós e arestas com proveniência.

As outras decisões da ADR-006 (indexação por AST, proveniência nas arestas, hooks graph-first, consultas de alto nível) não mudam. O SQLite + FTS5 da ADR-003 continua como opção futura, fora da Fase A.

## Consequências

- Evita-se uma dependência nova e uma segunda cópia dos dados; toda leitura reflete os arquivos como estão.
- Cada comando paga uma varredura completa; o NFR da spec (2.000 notas em menos de 1 s) é o gatilho para adicionar um cache.
- Repositórios inicializados antes desta mudança precisam rodar `tb init` de novo para versionar o vault, e um `.gitignore` que ignora `.toolbelt/vault/` (como o deste repositório) precisa perder essa linha à mão; o `tb init` só avisa.
- O `docs/arquitetura.md` e o README precisam deixar de descrever `.tb/vault/`, SQLite, sincronização e `tb md` como existentes.

## Alternativas consideradas

- **Manter a ADR-006/003 como estão e construir primeiro o índice SQLite.** Rejeitada: uma dependência, uma sincronização e estado que pode divergir, sem problema de desempenho medido.
- **Mover as invariantes para o vault.** Rejeitada: perde o formato estruturado e a verificação fail-closed (ADR-010).
- **Resolver ambiguidade como o Obsidian.** Rejeitada: a escolha muda em silêncio quando aparece uma nota de mesmo nome mais perto da raiz.

## Referências

- [Spec: vault Markdown compatível com o Obsidian, Fase A](../specs/vault.md)
- [ADR-003](003-sqlite-fts5-grafo.md), [ADR-006](006-inspiracoes-graphify-obsidian.md), [ADR-010](010-invariantes-fora-compactacao.md), [ADR-011](011-grafo-sob-demanda.md)
