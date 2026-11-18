🇧🇷 Português | [🇺🇸 English](../en/specs/vault.md)

# Spec: Vault Markdown compatível com Obsidian, Fase A (`tb vault`)

Autor: toolbelt · Status: Rascunho (rodada 2, após revisão em contexto novo) · Data: 2026-11-18 · Decisões de origem: [ADR-006](../adr/006-inspiracoes-graphify-obsidian.md), [ADR-003](../adr/003-sqlite-fts5-grafo.md) · Relacionadas: [ADR-010](../adr/010-invariantes-fora-compactacao.md), [ADR-011](../adr/011-grafo-sob-demanda.md)

## Visão geral

O ADR-006 decidiu que a memória de longo prazo do toolbelt seria um vault de Markdown compatível com o Obsidian: notas com frontmatter, wikilinks, backlinks e edição direta por humanos. Nada disso foi implementado. Esta spec define a **Fase A**: o formato da nota, a resolução de wikilinks, os comandos `tb vault` e as tools MCP equivalentes, e a integração com o `tb graph`.

A Fase A é deliberadamente pequena e **diverge** de partes aceitas do ADR-006 e do ADR-003 (ver Q1):
- o vault é a fonte de verdade e **não há índice derivado persistente** (nem SQLite/FTS5): cada comando lê o vault do disco;
- o vault fica em `.toolbelt/vault/`, não em `.tb/vault/`;
- as invariantes continuam em `invariants.yaml` e não migram para o vault;
- não há TUI (`tb md`), watcher nem sync bidirecional.

## Estado atual (2026-11-18)

| Item | Situação |
|------|----------|
| `VaultConfig{Path, SyncOnChange}` em `internal/config/config.go` | Existe, default `path: .toolbelt/vault`, `sync_on_change: true`; **nunca lido** |
| `.toolbelt/vault/` | Criado por `tb init`, vazio |
| `config.LocalCacheExcludePatterns` | Inclui `.toolbelt/vault/`: o vault é tratado como cache local e **não é versionado**, o que contradiz o ADR-006 |
| `tb mem` | Invariantes em `.toolbelt/invariants.yaml`; sem Markdown |
| `internal/graph/index_docs.go` | Não reconhece `[[wikilink]]` nem interpreta frontmatter: o frontmatter é tratado como texto comum, então caminhos nele viram arestas `mentions` e linhas `# ` (comentários YAML, ou dentro de blocos de código) viram seções |
| Persistência do grafo | `.toolbelt/graph.json`; não há SQLite/FTS5 no `go.mod` |
| Walk do grafo | Não pula `.toolbelt/`; `readSource` segue symlinks e não checa tipo de arquivo |
| `docs/arquitetura.md` §3 e §7, README | Descrevem vault, SQLite, `tb md` e sync como existentes, em `.tb/vault/` |

## Glossário

| Termo | Definição |
|-------|-----------|
| raiz | Diretório pai do `.toolbelt/` encontrado por `mem.LocateRepoToolbeltDir(cwd)`, com symlinks resolvidos. É a mesma raiz que o `tb mem` usa. |
| vault | Diretório `vault.path` da config, relativo à raiz (default `.toolbelt/vault`). |
| nota | Arquivo regular `.md` dentro do vault, fora dos diretórios ignorados (R3). |
| nome da nota | Nome do arquivo sem `.md`. |
| caminho da nota | Caminho relativo ao vault, com `/`, sem `.md`. Ex.: `decisoes/sqlite-adiado`. |
| wikilink | `[[alvo]]`, `[[alvo\|rótulo]]`, `[[alvo#heading]]`, `[[alvo#^bloco]]`, `[[#heading]]` (na própria nota) ou qualquer um deles com `!` na frente (embed). |
| alvo | Parte antes do primeiro `#` ou `\|`, sem espaços nas pontas e sem o sufixo `.md`. Pode ser vazio só quando há fragmento. |
| backlink | Nota B é backlink de A se B contém um wikilink que resolve para A. |
| link quebrado / ambíguo | Wikilink cujo alvo resolve para nenhuma / mais de uma nota (R8). |
| proveniência | Campo `source` do frontmatter: `manual`, `extracted` ou `inferred` (ADR-006). |
| dado de nota | Qualquer string derivada do conteúdo de uma nota: título, aliases, tags, corpo, trechos, alvos de link e mensagens que os citam. |

## Problema

1. Agentes e humanos não têm onde registrar **conhecimento** que não é invariante: decisões pequenas, fatos do projeto, contexto de uma investigação. O `invariants.yaml` é para regras curtas, checadas por `tb mem check`, e não serve para isso.
2. A documentação promete um vault que não existe, e o único pedaço implementado (o diretório) está no git exclude, então nada escrito nele seria compartilhado.
3. O grafo não entende wikilinks, então um vault aberto no Obsidian e o `tb graph` enxergariam grafos diferentes.

## Objetivos

- G1: notas legíveis e editáveis no Obsidian sem conversão. No subconjunto em que tb e Obsidian concordam (links não ambíguos, R8), o tb resolve cada wikilink para a mesma nota que o Obsidian; onde o Obsidian desempataria por caminho mais curto, o tb reporta erro. Uma fixture dourada (plano de verificação) é o oráculo.
- G2: CLI e MCP para criar, listar, ler, buscar e checar notas, e para listar links e backlinks.
- G3: `tb vault check` falha fechado (vault ausente, frontmatter inválido, links quebrados e ambíguos) e roda no `tb stack`.
- G4: o grafo, quando ligado, inclui notas e wikilinks com proveniência, inclusive em repositórios com `config.yaml` anterior a esta spec.
- G5: vault versionado por padrão.
- G6: nenhuma mudança de comportamento em `tb mem`, `invariants.yaml` ou `tb mem check`.

## Não-objetivos

- Substituir ou espelhar as invariantes no vault. As invariantes são um arquivo estruturado, fora da compactação (ADR-010, ainda `proposed`), com checagem fail-closed em `tb mem check`; um vault editável por humanos não oferece isso.
- Índice SQLite/FTS5 (ADR-003). Fica para quando o NFR de desempenho falhar. Sem índice, a questão "sync bidirecional" do ADR-006 deixa de existir na Fase A.
- TUI `tb md`, `tb md --obsidian`, watcher. `sync_on_change` fica reservado e sem efeito.
- Tags inline (`#tag` no corpo), Dataview, Canvas, plugins do Obsidian, `.obsidian/` como configuração lida pelo tb.
- Editar notas existentes pelo tb (renomear com atualização de links, mover).
- Wikilinks para arquivos fora do vault. Para isso existem `refs` (R4) e links Markdown relativos.
- Endurecer o walk do grafo fora do vault (symlinks e FIFOs em `docs/`, código etc.). É o mesmo problema do R21, mas fica registrado em Q5.
- Embeddings ou busca semântica.

## Jornadas de usuário (CUJs)

- **CUJ-1.** *Dado* um repositório com `tb init` feito, *quando* rodo `tb vault new "SQLite adiado" --type decision --tag storage`, *então* `.toolbelt/vault/sqlite-adiado.md` é criado com frontmatter válido e `source: manual`, e o caminho é impresso. Rodar de novo com o mesmo título falha sem sobrescrever.
- **CUJ-2.** *Dado* que abro `.toolbelt/vault/` no Obsidian, escrevo `[[sqlite-adiado]]` numa nota nova e salvo, *quando* rodo `tb vault backlinks sqlite-adiado`, *então* a nota nova aparece.
- **CUJ-3.** *Dado* uma nota com `[[nota-que-nao-existe]]`, *quando* rodo `tb vault check`, *então* o exit é 1 e a saída cita arquivo, linha e alvo.
- **CUJ-4.** *Dado* `TB_GRAPH=on` e um `config.yaml` gerado antes desta spec, *quando* rodo `tb graph index` e `tb graph explain note:sqlite-adiado`, *então* o grafo mostra a nota, seus links de saída e backlinks.
- **CUJ-5.** *Dado* um agente via MCP, *quando* chama `tb_vault_search` com `"sqlite"`, *então* recebe notas com caminho e metadados, e todo dado de nota vem num objeto `data`, redigido pelos padrões do R15.
- **CUJ-6.** *Dado* um repositório inicializado antes desta spec, *quando* rodo `tb init` de novo, *então* `.toolbelt/vault/` sai do `.git/info/exclude` e `.toolbelt/vault/.obsidian/` entra, sem perder linhas do usuário no arquivo.
- **CUJ-7.** *Dado* um agente via MCP, *quando* chama `tb_vault_new`, *então* a nota nasce com `source: inferred` e a `confidence` que o agente informou; o agente não consegue gravar `source: manual`.

## Plano técnico

```
humano (Obsidian/editor) ──┐                 ┌── agente (MCP tb_vault_*)
                           ▼                 ▼
                 .toolbelt/vault/*.md  ◄── tb vault (CLI)
                 (fonte de verdade, versionada)
                           │ leitura sob demanda (sem índice persistente)
                           ▼
          internal/vault: scan seguro → parse → resolve
                           │                    ▲
                           │          internal/mdparse (folha):
                           │          slugify, headings com cercas,
                           │          frontmatter, wikilinks
                           ▼                    ▼
   list/show/search/links/        internal/graph: VaultIndexer
   backlinks/check                (só com TB_GRAPH=on, ADR-011)
```

`internal/mdparse` não importa nada do projeto. `internal/vault` e `internal/graph` importam `mdparse`; o `VaultIndexer` importa `internal/vault`, e `internal/vault` **não** importa `internal/graph`. Isso evita o ciclo que surgiria se o vault usasse o `slugify` do grafo e o grafo usasse o parser do vault.

### Requisitos

**Localização e versionamento**

- R1: o vault é `vault.path`, relativo à raiz (glossário). O tb aplica `filepath.Clean`, junta com a raiz, resolve symlinks com `EvalSymlinks` e exige que o resultado fique dentro da raiz; senão, exit 2 com `vault_path_outside_repo`. O vault resolvido é a raiz do walk do R3. Vault inexistente: exit 1 com `vault_not_found` em todos os comandos, inclusive `check`.
- R2: `EnsureLocalGitExclude` passa a receber os padrões extras derivados da config: `<vault.path>/.obsidian/`. `.toolbelt/vault/` sai de `LocalCacheExcludePatterns`. `tb init` carrega a config (se existir) para calcular esses padrões. Como `upsertExcludeSection` reescreve a seção inteira, um `tb init` em repositório antigo aplica a troca (CUJ-6). Correção junto: hoje, sem o marcador de fim, `upsertExcludeSection` descarta tudo depois do marcador de início; passa a acrescentar um bloco novo e preservar o resto.
- R3: o scan percorre o vault com `filepath.WalkDir`, sem seguir symlinks. Nota é toda entrada com `Lstat` de arquivo regular e extensão `.md`. Ignora diretórios que começam com `.` (inclui `.obsidian/` e `.trash/`) e qualquer entrada que não seja arquivo regular ou diretório (symlink, FIFO, socket, device), reportada como `not_regular` no `check` (aviso). Uma nota acima de `vault.max_note_bytes` (default 256 KiB) não é lida e é reportada como `note_too_large` (erro). A leitura abre com `O_NOFOLLOW` e lê no máximo `max_note_bytes + 1` bytes.

**Formato da nota**

- R4: a nota pode começar com frontmatter YAML: a primeira linha é `---` (aceita `\r\n`) e o frontmatter termina na próxima linha que é só `---` ou `...`. Campos reconhecidos:

  | Campo | Tipo | Default |
  |-------|------|---------|
  | `title` | string | Primeiro H1 fora de cerca; sem H1, o nome da nota |
  | `type` | string, um de `vault.types` | `fact` |
  | `tags` | lista de string ou string única; cada tag casa `^[a-z0-9][a-z0-9/_-]*$` | `[]` |
  | `aliases` | lista de string ou string única | `[]` |
  | `source` | `manual`, `extracted` ou `inferred` | `manual` |
  | `confidence` | número em [0, 1] | ausente |
  | `created`, `updated` | RFC 3339 ou `YYYY-MM-DD` | ausente |
  | `refs` | lista de caminhos relativos à raiz, fora do vault | `[]` |

  Classificação dos problemas:

  | Caso | Código | Nível |
  |------|--------|-------|
  | YAML inválido, frontmatter sem fechamento, campo reconhecido com tipo errado | `bad_frontmatter` | erro |
  | `source` fora da lista, `type` fora de `vault.types`, `confidence` fora de [0, 1], data inválida | `bad_frontmatter` | erro |
  | `source: inferred` sem `confidence` | `missing_confidence` | erro |
  | `confidence` com `source` diferente de `inferred` | `unused_confidence` | aviso |
  | Tag fora do padrão | `bad_tag` | aviso (a tag é descartada) |
  | `refs` absoluto, com `..` que escapa da raiz, ou apontando para dentro do vault | `bad_ref` | erro |
  | `refs` inexistente | `ref_missing` | erro |

  Campos desconhecidos são preservados e ignorados. Nos comandos que não são `check`, uma nota com erro usa os defaults no campo afetado e ganha `warnings: [{code, line, message}]` na saída `--json`.
- R5: notas sem frontmatter são válidas. O tb nunca reescreve uma nota existente.
- R6: headings são linhas ATX (`#` a `######` seguidos de espaço) **fora** do frontmatter e de blocos de código cercados (` ``` ` ou `~~~`, com o mesmo marcador e comprimento ≥ ao de abertura no fechamento). O slug de heading é `mdparse.Slugify`, a mesma função que o `MarkdownIndexer` passa a usar (hoje `slugify` não exportado em `index_docs.go`).

**Wikilinks**

- R7: extração. Fora do frontmatter, de blocos cercados e de código inline (spans delimitados por crases, com a regra de mesmo comprimento do CommonMark), o parser procura `[[`, lê até o próximo `]]` na mesma linha e descarta o candidato se o miolo contém `[`, `]` ou quebra de linha. Embed é o `!` imediatamente antes de `[[`. No miolo:
  1. separa o rótulo no primeiro `|` que não está escapado; `\|` conta como separador (forma usada dentro de tabelas) e `\` é removido;
  2. separa o fragmento no primeiro `#` do que sobrou;
  3. alvo = o resto, com `TrimSpace` e sem sufixo `.md` (sem diferenciar maiúsculas).

  Alvo vazio com fragmento é link para a própria nota. Alvo vazio sem fragmento é descartado. Cada link guarda linha (1-based), coluna, alvo, fragmento, rótulo e `embed`.
- R8: resolução de um alvo `t` não vazio, na ordem, parando na primeira etapa com candidatos:
  1. se `t` contém `/`: a nota cujo caminho é `t`;
  2. as notas cujo nome é `t`;
  3. as notas que têm `t` em `aliases`.

  Todas as comparações ignoram maiúsculas (`strings.EqualFold`). Zero candidatos: `broken_link`. Um: resolvido. Mais de um: `ambiguous_link`. O Obsidian desempata pelo caminho mais curto; o tb não, porque essa escolha muda em silêncio quando alguém cria uma nota de mesmo nome mais perto da raiz, e o agente seguiria para a nota errada sem aviso.
- R9: fragmento `#heading` é comparado por slug (R6) com os headings da nota-alvo. Inexistente: `broken_fragment` (aviso, porque o Obsidian aceita). `#^bloco` não é verificado na Fase A.

**Comandos**

- R10: `tb vault new <título> [--type T] [--tag t]... [--source S --confidence C] [--dir sub] [--body -|texto]`:
  - nome do arquivo: `mdparse.Slugify(título)` + `.md`, em `<vault>/<sub>/`;
  - `--dir` deve ser relativo; o tb cria os diretórios que faltam um a um, com `Lstat` em cada componente, recusando symlink, e confere que o diretório final, resolvido, fica dentro do vault; senão, exit 2 com `dir_outside_vault`;
  - grava com `O_CREATE|O_EXCL|O_NOFOLLOW`, permissão `0644`; se existir, exit 1 com `note_exists` e o caminho;
  - frontmatter: `title`, `type`, `tags` (se houver), `source`, `confidence` (se houver), `created` (RFC 3339 UTC); corpo: `# <título>` seguido do `--body` (`-` lê o stdin, até `max_note_bytes`);
  - título e corpo passam por `mem.RedactText` antes de gravar; se algo mudou, grava o texto redigido e a saída traz `redacted: true`;
  - na CLI, `--source` default é `manual`; `inferred` exige `--confidence`.
- R11: `tb vault list [--type T] [--tag t] [--source S]` lista caminho, título, tipo, tags, `source` e `updated` (só do frontmatter; o mtime não entra na saída), ordenado por caminho.
- R12: `tb vault show <alvo>` resolve `<alvo>` pelas regras R7–R8 (aceita também o caminho da nota) e imprime frontmatter normalizado e corpo. Com `--json`, inclui `links` (com estado) e `backlinks`.
- R13: `tb vault search <query> [--limit N]` faz busca por substring, sem diferenciar maiúsculas e acentos, em título, aliases, tags e corpo, com a normalização de `normalizeForSearch` (hoje não exportada, em `internal/mem/validate.go`; passa para `mdparse.NormalizeForSearch` e `mem` a reusa). A busca percorre o texto original runa a runa, comparando a forma normalizada de cada runa, para que os offsets do acerto valham no texto original. O trecho tem até 200 runas ao redor do primeiro acerto e é recortado **depois** da redação do texto inteiro (R15). Ordem: caminho, depois linha do primeiro acerto. `--limit` default 20.
- R14: `tb vault links <alvo>` lista os wikilinks de saída com estado (`ok`, `broken`, `ambiguous`, `broken_fragment`); `tb vault backlinks <alvo>` lista as notas que apontam para a nota, com linha. Ordem: caminho, linha, coluna.
- R15: saídas que contêm dado de nota passam por `mem.RedactText` aplicado ao texto inteiro antes de qualquer recorte, na CLI e no MCP. O `RedactText` cobre só os padrões já listados em `internal/mem/redact.go` (tokens GitHub `ghp_`/`gho_`/`github_pat_`, AWS `AKIA`, chave PEM, `Bearer`, Slack `xox`, `*_TOKEN=`). Esta spec acrescenta `ghu_`, `ghs_`, `sk-` (com ≥ 20 caracteres) e `password|passwd|secret` seguidos de `=` ou `:`. O que fica fora desses padrões é risco residual (Modelo de ameaças).
- R16: `tb vault check [--strict]` valida o vault inteiro. Erros (exit 1): `vault_not_found`, `bad_frontmatter`, `missing_confidence`, `bad_ref`, `ref_missing`, `note_too_large`, `broken_link`, `ambiguous_link`. Avisos (exit 0): `not_regular`, `bad_tag`, `unused_confidence`, `broken_fragment`, `orphan_note` (sem links de entrada nem de saída). `--strict` promove avisos a erros. Vault existente e vazio: exit 0 com `notes: 0`.
- R17: todos os comandos aceitam `--json`, com chaves em snake_case, e as mesmas entradas geram a mesma saída byte a byte (NFR).

**MCP e stack**

- R18: tools `tb_vault_new`, `tb_vault_list`, `tb_vault_show`, `tb_vault_search`, `tb_vault_links`, `tb_vault_backlinks` e `tb_vault_check`, com schemas tipados, registradas em `internal/registry/parity.go` para manter a paridade CLI/MCP.
  - **Separação de dado:** todo dado de nota (glossário) vai num objeto `data` de cada item; fora dele ficam só campos gerados pelo tb (caminho, códigos, linhas, estados, contagens). As mensagens do `check` citam o alvo só dentro de `data`.
  - **Proveniência:** `tb_vault_new` não aceita `source`; grava sempre `source: inferred` e exige `confidence` (CUJ-7).
- R19: `tb stack` ganha a op `vault` (roda R16), escolhida como as outras, por argumento posicional (`tb stack verify guard vault`). As listas de ops em `cmd/tb/stack.go` e o schema da tool `tb_stack` em `internal/registry/parity.go` incluem a op.

**Grafo (só com `TB_GRAPH=on`, ADR-011)**

- R20: novo `VaultIndexer` com `Name() == "vault"`, categoria `doc`, que casa arquivos `.md` sob o vault resolvido (R1). O `MarkdownIndexer` passa a ignorar esses arquivos, para que cada nota tenha um único nó. `allowsIndexer` trata `vault` como habilitado sempre que `markdown` está em `Docs`, para que repositórios com `config.yaml` antigo (que lista `markdown, html, txt`) não percam as notas do grafo (G4); `vault` também entra no default de `Docs`.
- R21: o `VaultIndexer` **não** usa `readSource`: lê pelo `internal/vault` (R3), com `Lstat`, só arquivo regular, `O_NOFOLLOW` e limite de tamanho. Symlink, FIFO e arquivo grande no vault nunca são lidos pelo grafo, então um repositório clonado não consegue, por um symlink versionado no vault, fazer o `graph.json` (e as tools `tb_graph_*`) carregar conteúdo de fora do repositório.
- R22: cada nota vira um nó `NodeNote` (novo) com ID `note:<caminho da nota>`, `Path` do arquivo e `Title` do R4; `Source` e `Confidence` vêm do frontmatter (`manual` → 1.0, `extracted` → 0.9, `inferred` → o `confidence`). Headings (R6, fora de cerca e de frontmatter) viram `NodeSection` com ID `section:<arquivo>#<slug>`.
- R23: wikilink resolvido vira aresta `EdgeLinks` de `note:A` para `note:B`, ou para `section:<arquivo de B>#<slug>` com fragmento, `Source: "extracted"`, confiança 0.9. Links quebrados e ambíguos não viram aresta. O corpo (sem frontmatter) continua gerando links Markdown, autolinks e menções como no `MarkdownIndexer`. Cada `refs` válido vira aresta `EdgeMentions` para `file:<ref>`, confiança 0.95.
- R24: tags viram nós `NodeTag` (`tag:<tag>`) com aresta `EdgeTagged` (nova) da nota para a tag.
- R25: consultas sobre notas:
  - `tb graph explain` e `tb graph path` aceitam IDs `note:` e `tag:`; `tb graph search` inclui nós `note` e `tag`;
  - `Store.Impact(file)` ganha um passo: depois das arestas `defines`/`calls` de hoje, inclui as notas com aresta `EdgeMentions` ou `EdgeLinks` cujo alvo é `file:<file>` ou uma seção dele; `findPackageOrFileIDs` passa a aceitar também `NodeNote`, para `impact` sobre uma nota listar as notas que a citam;
  - mudanças em `cmd/tb/graph.go` e `internal/registry/graph.go` limitadas a aceitar esses IDs e tipos; nenhum flag novo.

### Configuração

```yaml
vault:
  path: .toolbelt/vault        # existente
  sync_on_change: true         # existente (default atual true); reservado, sem efeito na Fase A
  max_note_bytes: 262144       # novo
  types: [fact, decision, context, log]  # novo
graph:
  docs: [markdown, vault, html, txt]     # vault novo no default; com só markdown, vault fica implícito (R20)
```

`VaultConfig.Validate` rejeita `path` absoluto ou com `..` que escape da raiz, `max_note_bytes` ≤ 0 ou acima de 4 MiB, e `types` vazio, com valores repetidos ou fora de `^[a-z][a-z0-9_-]*$`.

## Modelo de ameaças

| Ameaça | Mitigação |
|--------|-----------|
| Nota escrita por agente ou vinda de repositório clonado instrui outro agente (injeção via MCP) | R18 (todo dado de nota dentro de `data`), R15. Risco residual: o agente ainda lê o texto e pode obedecê-lo. |
| Combinação "dado privado + conteúdo não confiável + saída externa" (ADR-010) | O tb não tem saída externa (local-first). O `tb_vault_*` entrega conteúdo não confiável ao agente, e é o harness do agente que decide se ele tem saída externa. Fica registrado como risco do harness, não do tb. |
| Segredo colado numa nota chega a um prompt | R10 (redação na escrita), R15 (redação na leitura, antes do recorte). Risco residual: segredos fora dos padrões do R15. |
| `vault.path` ou `--dir` apontando para fora; symlink em componente intermediário | R1 (`EvalSymlinks` da raiz do vault), R10 (`Lstat` por componente, `O_NOFOLLOW`) |
| Symlink versionado no vault para `~/.ssh/...` lido pelo grafo e devolvido por `tb_graph_*` | R21 |
| Nota gigante ou FIFO trava o scan, o `tb graph index` ou o `tb stack` | R3, R21 (só arquivo regular, leitura limitada) |
| `tb vault new` sobrescreve nota humana | R10 (`O_EXCL`) |
| Nota de agente se passa por conhecimento humano (`source: manual`, confiança 1.0) | R18 (MCP grava sempre `inferred`). Risco residual: um agente com acesso a arquivos ou à CLI pode escrever `manual` à mão; a proveniência é declarativa, não assinada. |
| Link resolvido diferente do Obsidian leva o agente à nota errada | R8 (ambiguidade é erro) |
| `vault.path` com erro de digitação faz o `check` passar com zero notas | R1, R16 (`vault_not_found` é erro) |
| Config do Obsidian com dados pessoais versionada sem querer | R2 |

## Requisitos não funcionais (NFRs)

- **Desempenho:** `tb vault check` num vault com 2 000 notas de 4 KiB roda em menos de 1 s numa máquina de desenvolvimento. Se falhar, a Fase B introduz um cache por fingerprint (como o `graph.json`) antes de considerar SQLite.
- **Determinismo:** as mesmas notas geram a mesma saída `--json`, byte a byte (ordens definidas em R11, R13, R14; nada derivado de mtime).
- **Local-first:** nenhum comando fala com a rede (invariante do projeto).

## Alternativas

- **Migrar `tb mem` para notas do vault.** Rejeitada: perde o formato estruturado e o fail-closed do `tb mem check`, e invariantes imutáveis passariam a ser editáveis em qualquer editor.
- **Índice SQLite/FTS5 desde já (ADR-003, ADR-006).** Adiada: dependência nova, um sync para manter e um estado que pode divergir, sem problema de desempenho medido.
- **Resolver ambiguidade como o Obsidian.** Rejeitada pelo motivo em R8.
- **Vault fora do repositório (ex.: `$XDG_DATA_HOME`).** Rejeitada para o default: o objetivo é compartilhar conhecimento pelo git. R1 exige caminho dentro da raiz; vault pessoal fica para reavaliação se houver demanda.
- **Exportar `slugify` do grafo e importar o grafo no vault.** Rejeitada: ciclo de import com o `VaultIndexer`. Daí o pacote folha `mdparse`.

## Questões em aberto

- Q1: esta spec contraria decisões aceitas do ADR-006 (`.tb/vault/`, índice SQLite, sync, `tb md`) e do ADR-003 (SQLite+FTS5 como índice derivado). Precisa de um ADR novo que as substitua parcialmente, ou de adendos nos dois. Numeração: as branches `feature/graph-explain-path` e `feature/egm-native` propõem, cada uma, um ADR-014 diferente; o próximo número livre depende de qual entra primeiro. De passagem: o ADR-011 diz `accepted` no arquivo e `proposed` no índice `docs/adr/README.md`.
- Q2: wikilink para invariante (`[[inv:<id>]]`) resolvendo para `invariants.yaml`? Proposta: Fase B.
- Q3: `tb review` e `tb drift` deveriam ler notas `type: decision` ligadas ao ticket? Fora da Fase A.
- Q4: `orphan_note` como aviso gera ruído em vaults novos? Medir no dogfooding.
- Q5: o walk do grafo fora do vault tem a mesma fraqueza do R21 (`readSource` segue symlinks e lê FIFOs). Vale um fix separado, com o mesmo leitor seguro, para todos os indexadores.

## Plano de Implementação Detalhado

| Arquivo | Ação | Motivo |
|---------|------|--------|
| `internal/mdparse/slug.go`, `headings.go`, `frontmatter.go`, `wikilink.go`, `normalize.go` (+ testes) | Create | Pacote folha (R4, R6, R7, R13) |
| `internal/graph/index_docs.go` | Modify | Usar `mdparse.Slugify` e headings com cerca; `MarkdownIndexer` ignora o vault (R6, R20) |
| `internal/mem/validate.go` | Modify | Reusar `mdparse.NormalizeForSearch` |
| `internal/mem/redact.go`, `redact_test.go` | Modify | Padrões novos (R15) |
| `internal/vault/scan.go`, `read.go` | Create | Raiz, walk e leitura seguros (R1, R3, R21) |
| `internal/vault/note.go`, `resolve.go` | Create | Nota, defaults, classificação, resolução (R4, R5, R8, R9) |
| `internal/vault/vault.go`, `search.go`, `check.go` | Create | `New`, `List`, `Show`, `Search`, `Links`, `Backlinks`, `Check` (R10–R16) |
| `internal/vault/*_test.go`, `internal/vault/testdata/` | Create | Fixtures, incluindo a fixture dourada do G1 |
| `internal/config/config.go`, `config_test.go` | Modify | `max_note_bytes`, `types`, `VaultConfig.Validate`, `vault` no default de `Docs` |
| `internal/config/exclude.go`, `exclude_test.go` | Modify | Padrões extras, tirar `vault/`, marcador de fim ausente (R2) |
| `cmd/tb/main.go`, `cmd/tb/init_exclude_test.go` | Modify | `init` carrega config para o exclude; teste de upgrade (CUJ-6) |
| `cmd/tb/vault.go`, `cmd/tb/vault_test.go` | Create | Grupo cobra (R10–R17) |
| `cmd/tb/json_snake_case_test.go` | Modify | Amostras das saídas `tb vault` |
| `internal/registry/parity.go`, `parity_test.go` | Modify | Tools `tb_vault_*` (R18); op `vault` no schema de `tb_stack` (R19) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | Op `vault` (R19) |
| `internal/graph/graph.go`, `graph_test.go` | Modify | `NodeNote`, `NodeTag`, `EdgeTagged`; passo novo em `Impact`; `findPackageOrFileIDs` (R22–R25) |
| `internal/graph/indexer.go` | Modify | `allowsIndexer` com `vault` implícito (R20) |
| `internal/graph/index_vault.go`, `index_vault_test.go` | Create | `VaultIndexer` (R20–R24) |
| `cmd/tb/graph.go`, `internal/registry/graph.go` | Modify | Aceitar IDs `note:`/`tag:` (R25) |
| `docs/en/specs/vault.md` | Create | Espelho EN desta spec (o teste `TestDocsEnglishMirrorComplete` exige) |
| `docs/cli.md`, `docs/arquitetura.md` §3 e §7, `README.md` (+ `docs/en/`, `README.en.md`) | Modify | Trocar `.tb/vault` por `.toolbelt/vault`; marcar SQLite, `tb md` e sync como planejados; documentar `tb vault` |
| ADR novo ou adendos em `docs/adr/003-…` e `006-…` (+ EN) | Create/Modify | Q1 |
| `CHANGELOG.md` | Modify | Entrada não lançada |

### Plano de verificação

- **Parser (`mdparse`):** um teste por forma do glossário: `[[a]]`, `[[a|b]]`, `[[a\|b]]` (em tabela), `[[a#h]]`, `[[a#^x]]`, `[[#h]]`, `![[a]]`, `[[pasta/a]]`, `[[a.md]]`, `[[ a ]]`, `[[A]]` contra `a.md`; ignorados: dentro de ` ``` `, de `~~~`, de código inline com uma e duas crases, no frontmatter; descartados: `[[]]`, `[[a]` + quebra de linha. Headings dentro de cerca e comentários YAML não viram heading. CRLF: linhas e frontmatter corretos.
- **Resolução:** nome único; caminho; alias; dois `a.md` em pastas diferentes (`ambiguous_link`); alias igual ao nome de outra nota (a etapa 2 vence a 3); mesmo alias em duas notas (`ambiguous_link`); fragmento existente e inexistente.
- **Fixture dourada (G1):** vault em `internal/vault/testdata/obsidian/` com o resultado esperado de cada link em `expected.json`, conferido uma vez no Obsidian real (registro em `docs/campo/dogfood.md`); o teste compara o tb com o `expected.json`.
- **Frontmatter:** cada linha da tabela de classificação do R4.
- **Segurança:** symlink para fora do vault (scan e grafo); diretório intermediário symlink em `--dir`; FIFO `x.md` no vault (scan, `tb graph index` e `tb stack vault` terminam); nota acima do limite; `vault.path: ../x`; `vault.path` inexistente (`vault_not_found`, exit 1); segredo no `--body` (gravado redigido); segredo numa nota editada à mão, inclusive cortado pela janela do trecho (saída redigida em `show`, `search` e MCP); `tb_vault_new` gravando `inferred` mesmo se o agente tentar `source`.
- **Exclude:** `init` em repositório com a seção antiga troca `vault/` por `vault/.obsidian/` (checagem por linha inteira, não substring, porque `.toolbelt/vault/.obsidian/` contém `.toolbelt/vault/`); arquivo sem marcador de fim preserva as linhas do usuário; `vault.path` customizado gera o padrão certo.
- **Grafo:** com `TB_GRAPH=on` e um `config.yaml` antigo (`docs: [markdown, html, txt]`), cada nota gera um único nó `note:` e nenhum `file:`; arestas de wikilink, fragmento, `refs` e tag; `tb graph impact internal/x.go` lista a nota que cita o arquivo; `explain note:` e `path` entre duas notas; com o grafo desligado, `tb vault` funciona igual.
- **Regressão:** testes de `tb mem`, `tb graph`, `json_snake_case`, paridade do registry e espelho de docs passam.
- **Desempenho:** benchmark Go com 2 000 notas geradas (NFR).
