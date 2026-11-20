[🇺🇸 English](../../specs/vault.md) | 🇧🇷 Português

# Spec: Vault Markdown compatível com Obsidian, Fase A (`tb vault`)

Autor: toolbelt · Status: Rascunho (rodada 3: atualizada contra a main, revisão em contexto novo aplicada, Q1 resolvida pela [ADR-016](../adr/016-vault-fase-a.md); Q4 aberta); fatias 1 (mdparse) e 2 (vault só leitura) de implementação feitas · Data: 2026-11-20 · Decisão: [ADR-016](../adr/016-vault-fase-a.md) · Decisões de origem: [ADR-006](../adr/006-inspiracoes-graphify-obsidian.md), [ADR-003](../adr/003-sqlite-fts5-grafo.md) · Relacionadas: [ADR-010](../adr/010-invariantes-fora-compactacao.md), [ADR-011](../adr/011-grafo-sob-demanda.md)

## Visão geral

O ADR-006 decidiu que a memória de longo prazo do toolbelt seria um vault de Markdown compatível com o Obsidian: notas com frontmatter, wikilinks, backlinks e edição direta por humanos. Nada disso foi implementado. Esta spec define a **Fase A**: o formato da nota, a resolução de wikilinks, os comandos `tb vault` e as tools MCP equivalentes, e a integração com o `tb graph`.

A Fase A é deliberadamente pequena e **diverge** de partes aceitas do ADR-006 e do ADR-003 (ver Q1 e a [ADR-016](../adr/016-vault-fase-a.md)):
- o vault é a fonte de verdade e **não há índice derivado persistente** (nem SQLite/FTS5): cada comando lê o vault do disco;
- o vault fica em `.toolbelt/vault/`, não em `.tb/vault/`;
- as invariantes continuam em `invariants.yaml` e não migram para o vault;
- não há TUI (`tb md`), watcher nem sync bidirecional.

## Estado atual (2026-11-20)

| Item | Situação |
|------|----------|
| `VaultConfig{Path, SyncOnChange}` em `internal/config/config.go` | Existe, default `path: .toolbelt/vault`, `sync_on_change: true`; **nunca lido** |
| `.toolbelt/vault/` | Criado por `tb init`, vazio |
| `config.LocalCacheExcludePatterns` | Inclui `.toolbelt/vault/`: o vault é tratado como cache local e **não é versionado**, o que contradiz o ADR-006 |
| `.gitignore` deste repositório | Ignora `.toolbelt/vault/`, então mesmo sem o exclude o vault não seria versionado aqui |
| `tb mem` | Invariantes em `.toolbelt/invariants.yaml`; sem Markdown |
| `internal/graph/index_docs.go` | Não reconhece `[[wikilink]]` nem interpreta frontmatter: o frontmatter é tratado como texto comum, então caminhos nele viram arestas `mentions` e linhas `# ` (comentários YAML, ou dentro de blocos de código) viram seções |
| Persistência do grafo | `.toolbelt/graph.json`; não há SQLite/FTS5 no `go.mod` |
| Walk do grafo | Não pula `.toolbelt/`; `readSource` segue symlinks e não checa tipo de arquivo |
| `docs/arquitetura.md` §3 e §7, README | Descrevem vault, SQLite, `tb md` e sync como existentes, em `.tb/vault/` |

## Glossário

| Termo | Definição |
|-------|-----------|
| raiz | Diretório pai do `.toolbelt/` encontrado por `mem.LocateRepoToolbeltDir(cwd)`, com symlinks resolvidos (R0). É a mesma raiz que o `tb mem` usa. |
| vault | Diretório `vault.path` da config, relativo à raiz (default `.toolbelt/vault`). |
| nota | Arquivo regular `.md` dentro do vault, fora dos diretórios ignorados (R3). |
| anexo | Qualquer outro arquivo regular dentro do vault, fora dos diretórios ignorados (R3, R8). |
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

- G1: notas legíveis e editáveis no Obsidian sem conversão. No subconjunto em que tb e Obsidian concordam (links não ambíguos, R8), o tb resolve cada wikilink para a mesma nota que o Obsidian; onde o Obsidian desempataria por caminho mais curto, o tb reporta erro. Uma fixture dourada conferida no Obsidian real (plano de verificação, Q4) é o oráculo.
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
- Wikilinks para arquivos fora do vault. Para isso existem links Markdown relativos; um campo `refs` no frontmatter fica para a Fase B.
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

**Localização, config e versionamento**

- R0: a raiz é o pai do diretório `.toolbelt/` devolvido por `mem.LocateRepoToolbeltDir(cwd)`, com symlinks resolvidos; no MCP, `cwd` é o argumento `cwd` da tool, nunca o do servidor. A config é lida de `<raiz>/.toolbelt/config.yaml` explicitamente (como no `internal/router`), nunca do cwd do processo. Uma raiz sem `.toolbelt/` é exit 2 com `no_toolbelt_dir`.
- R1: o vault é `vault.path`, relativo à raiz. O tb aplica `filepath.Clean`, junta com a raiz, resolve symlinks com `EvalSymlinks` e exige que o resultado fique dentro da raiz; senão, exit 2 com `vault_path_outside_repo`. O vault resolvido é a raiz do walk do R3. Vault inexistente: exit 1 com `vault_not_found` em todos os comandos, inclusive `check`.
- R2: `EnsureLocalGitExclude` passa a receber os padrões extras derivados da config: `<vault.path>/.obsidian/`. `.toolbelt/vault/` sai de `LocalCacheExcludePatterns`. `tb init` carrega a config (se existir) para calcular esses padrões, cria `vault.path` quando não é o default (o `InitDetailed` fixa `vault` e retorna cedo quando já existe config), e o log dele lista também os padrões extras. Como `upsertExcludeSection` reescreve a seção inteira, um `tb init` em repositório antigo aplica a troca (CUJ-6). Correção junto: hoje, com marcador de início e sem o de fim, `upsertExcludeSection` mantém só o texto antes do marcador de início e descarta as linhas do usuário depois dele. Passa a remover a linha de início órfã e, depois dela, só as linhas iguais a um padrão que o tb grava ou já gravou (listas atual e anteriores), manter todas as outras e acrescentar um bloco novo. Um `.gitignore` que ignora o vault não é editado; o `tb init` avisa `vault_gitignored` (o `.gitignore` deste repositório perde a linha na mesma mudança).
- R3: o scan percorre o vault com `filepath.WalkDir`, sem seguir symlinks. Ignora diretórios que começam com `.` (inclui `.obsidian/` e `.trash/`). Candidata a nota é toda entrada com `Lstat` de arquivo regular e extensão `.md`; os outros arquivos regulares são anexos (R8). Qualquer entrada que não seja arquivo regular nem diretório (symlink, FIFO, socket, device) é reportada como `not_regular` no `check` (aviso). A leitura abre com `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`, faz `fstat` no descritor e exige o arquivo regular que o walk listou (`os.SameFile`: uma troca por FIFO, symlink ou diretório intermediário depois da listagem é recusada, também no Windows, onde `O_NOFOLLOW` não existe), e lê no máximo `max_note_bytes + 1` bytes. Uma nota acima de `vault.max_note_bytes` (default 256 KiB) é reportada como `note_too_large` (erro) e não é interpretada. Um diretório ou arquivo ilegível é aviso `unreadable` e a varredura continua. Uma entrada `.md` não interpretada (grande demais, não regular, ilegível) continua existindo para a resolução de links, então links para ela não são `broken_link`; os fragmentos dela não são verificados.

**Formato da nota**

- R4: a nota pode começar com frontmatter YAML: a primeira linha é `---` (aceita `\r\n`, um BOM UTF-8 no início e espaços ou tabs no fim) e o frontmatter termina na próxima linha que é só `---` ou `...` (com espaços ou tabs no fim). Um arquivo que começa com uma quebra temática `---` é lido como frontmatter, como no Obsidian. Campos reconhecidos:

  | Campo | Tipo | Default |
  |-------|------|---------|
  | `title` | string | Primeiro H1 fora de cerca; sem H1, o nome da nota |
  | `type` | string, um de `vault.types` | `fact` |
  | `tags` | lista de string ou string única; cada tag casa `^[a-z0-9][a-z0-9/_-]*$` | `[]` |
  | `aliases` | lista de string ou string única (só metadado e busca, R8) | `[]` |
  | `source` | `manual`, `extracted` ou `inferred` | `manual` |
  | `confidence` | número em [0, 1] | ausente |
  | `created`, `updated` | RFC 3339 ou `YYYY-MM-DD` | ausente |

  Classificação dos problemas:

  | Caso | Código | Nível |
  |------|--------|-------|
  | YAML inválido, frontmatter sem fechamento, campo reconhecido com tipo errado | `bad_frontmatter` | erro |
  | `source` fora da lista, `type` fora de `vault.types`, `confidence` fora de [0, 1], data inválida | `bad_frontmatter` | erro |
  | `source: inferred` sem `confidence` | `missing_confidence` | erro |
  | `confidence` com `source` diferente de `inferred` | `unused_confidence` | aviso |
  | Tag fora do padrão | `bad_tag` | aviso (a tag é descartada) |

  Campos desconhecidos são preservados e ignorados. Nos comandos que não são `check`, uma nota com erro usa os defaults no campo afetado e ganha `warnings: [{code, line, message}]` na saída `--json`.
- R5: notas sem frontmatter são válidas. O tb nunca reescreve uma nota existente.
- R6: headings são linhas ATX (`#` a `######` seguidos de espaço) **fora** do frontmatter e de blocos de código cercados (` ``` ` ou `~~~`, com o mesmo marcador e comprimento ≥ ao de abertura no fechamento). O slug de heading é `mdparse.Slugify`, a mesma função que o `MarkdownIndexer` passa a usar (hoje `slugify` não exportado em `index_docs.go`).

**Wikilinks**

- R7: extração. Fora do frontmatter, de blocos cercados e de código inline (spans delimitados por crases, com a regra de mesmo comprimento do CommonMark), o parser procura `[[`, lê até o próximo `]]` na mesma linha e descarta o candidato se o miolo contém `[`, `]` ou quebra de linha. Embed é o `!` imediatamente antes de `[[`. No miolo:
  1. separa o rótulo no primeiro `|` que não está escapado; `\|` conta como separador (forma usada dentro de tabelas) e `\` é removido;
  2. separa o fragmento no primeiro `#` do que sobrou;
  3. alvo = o resto, com `TrimSpace` e sem sufixo `.md` (sem diferenciar maiúsculas).

  Alvo vazio com fragmento é link para a própria nota. Alvo vazio sem fragmento é descartado. Cada link guarda linha (1-based), coluna (1-based, em runas, do `!` num embed e do primeiro `[` nos outros casos), alvo, fragmento, rótulo e `embed`. Uma crase escapada com barra invertida não abre span de código. Limites da Fase A: spans de código e wikilinks são reconhecidos dentro de uma linha, e cercas dentro de itens de lista ou de citações não são reconhecidas, então os links dentro delas são extraídos.
- R8: resolução de um alvo `t` não vazio, sem diferenciar maiúsculas (minúsculas Unicode):
  - **Anexo:** se nenhuma nota casa com `t`, e um anexo casa com ele ou a extensão dele tem de 1 a 8 letras ou dígitos (por exemplo `img.png`, `doc.pdf`, mas não `Dr. Smith` nem `v2.0 release`), é um link para anexo, então uma nota chamada `v1.2` continua vencendo e um nome de nota com ponto que não existe continua `broken_link`. Vários anexos casando são aviso `ambiguous_attachment`. Resolve contra os arquivos regulares do vault que não são nota, pelas mesmas duas etapas abaixo; zero candidatos é `missing_attachment` (aviso), nunca erro, porque vaults do Obsidian embutem com frequência arquivos que o tb não gerencia.
  - **Nota**, na ordem, parando na primeira etapa com candidatos:
    1. se `t` contém `/`: as notas cujo caminho é `t` ou termina em `/t` (o casamento por caminho parcial do Obsidian);
    2. senão, as notas cujo nome é `t`.
  - Zero candidatos: `broken_link`. Um: resolvido. Mais de um: `ambiguous_link`. O Obsidian desempata pelo caminho mais curto; o tb não, porque essa escolha muda em silêncio quando alguém cria uma nota de mesmo nome mais perto da raiz, e o agente seguiria para a nota errada sem aviso.
  - `aliases` não resolvem links: no Obsidian elas só alimentam o autocomplete, que grava `[[nota|alias]]`.
- R9: fragmento `#heading` é comparado por slug (R6) com os headings da nota-alvo. Inexistente: `broken_fragment` (aviso, porque o Obsidian aceita). `#^bloco` não é verificado na Fase A.

**Comandos**

- R10: `tb vault new <título> [--type T] [--tag t]... [--source S --confidence C] [--dir sub] [--body -|texto]`:
  - o título não pode ter caracteres de controle (inclusive quebra de linha): exit 2 com `bad_title`;
  - título e corpo passam antes pela redação do vault (R15); a saída traz `redacted: true` quando algo mudou;
  - nome do arquivo: `mdparse.Slugify(título redigido)`, cortado em 80 bytes numa fronteira de runa, + `.md`, em `<vault>/<sub>/`; slug vazio é exit 2 com `bad_title` (nunca o `section` de reserva do grafo);
  - `--dir` deve ser relativo; o tb cria os diretórios que faltam um a um, com `Lstat` em cada componente, recusando symlink, e confere que o diretório final, resolvido, fica dentro do vault; senão, exit 2 com `dir_outside_vault`;
  - grava com `O_CREATE|O_EXCL|O_NOFOLLOW`, permissão `0644`; se existir, exit 1 com `note_exists` e o caminho;
  - o frontmatter sai de `yaml.Marshal` de uma struct (nunca de interpolação de string): `title`, `type`, `tags` (se houver), `source`, `confidence` (se houver), `created` (RFC 3339 UTC); corpo: `# <título>` seguido do `--body` (`-` lê o stdin, até `max_note_bytes`);
  - na CLI, `--source` default é `manual`; `inferred` exige `--confidence`.
- R11: `tb vault list [--type T] [--tag t] [--source S]` lista caminho, título, tipo, tags, `source` e `updated` (só do frontmatter; o mtime não entra na saída), ordenado por caminho.
- R12: `tb vault show <alvo>` resolve `<alvo>` pelas regras R7–R8: o texto de um wikilink com ou sem `[[ ]]`, rótulo e fragmento, ou o caminho da nota com ou sem `.md`; duas notas cujos caminhos só diferem em maiúsculas são `ambiguous_link`. Ele e imprime frontmatter normalizado e corpo. Com `--json`, inclui `links` (com estado) e `backlinks`.
- R13: `tb vault search <query> [--limit N]` primeiro redige o texto inteiro de cada nota (R15) e depois faz busca por substring, sem diferenciar maiúsculas e acentos, no título, aliases, tags e corpo redigidos, para que uma consulta nunca consiga sondar um segredo pela contagem ou ordem dos acertos. A normalização é a de `normalizeForSearch` (hoje não exportada, em `internal/mem/validate.go`; passa para `mdparse.NormalizeForSearch` e `mem` a reusa). A busca percorre o texto redigido runa a runa, comparando a forma normalizada de cada runa, para que os offsets do acerto valham nesse texto. O trecho tem até 200 runas ao redor do primeiro acerto. Ordem: caminho, depois linha do primeiro acerto. `--limit` default 20.
- R14: `tb vault links <alvo>` lista os wikilinks de saída com estado (`ok`, `broken`, `ambiguous`, `broken_fragment`, `attachment`, `missing_attachment`); `tb vault backlinks <alvo>` lista as notas que apontam para a nota, com linha. Ordem: caminho, linha, coluna.
- R15: saídas que contêm dado de nota passam pela redação do vault, aplicada ao texto inteiro antes de qualquer recorte ou busca, na CLI e no MCP. A redação do vault é uma lista própria no `internal/vault`: `mem.SecretPatterns()` mais `ghu_`, `ghs_`, `sk-` (com ≥ 20 caracteres) e `password|passwd|secret` seguidos de `=` ou `:`. Os padrões do próprio `mem` não mudam, então as saídas do `tb mem` e do `tb egm` continuam iguais (G6). O que fica fora desses padrões é risco residual (Modelo de ameaças).
- R16: `tb vault check` valida o vault inteiro. Erros (exit 1): `vault_not_found`, `bad_frontmatter`, `missing_confidence`, `note_too_large`, `broken_link`, `ambiguous_link`. Avisos (exit 0): `not_regular`, `unreadable`, `bad_tag`, `unused_confidence`, `broken_fragment`, `missing_attachment`, `ambiguous_attachment`. Vault existente e vazio: exit 0 com `notes: 0`.
- R17: todos os comandos aceitam `--json`, com chaves em snake_case, e as mesmas entradas geram a mesma saída byte a byte (NFR).

**MCP e stack**

- R18: tools `tb_vault_new`, `tb_vault_list`, `tb_vault_show`, `tb_vault_search`, `tb_vault_links`, `tb_vault_backlinks` e `tb_vault_check`, com schemas tipados e `cwd` absoluto obrigatório (R0), registradas em `internal/registry/parity.go` para manter a paridade CLI/MCP.
  - **Separação de dado:** todo dado de nota (glossário) vai num objeto `data` de cada item; fora dele ficam só campos gerados pelo tb (caminho, códigos, linhas, estados, contagens). As mensagens do `check` citam o alvo só dentro de `data`.
  - **Proveniência:** `tb_vault_new` não aceita `source`; grava sempre `source: inferred` e exige `confidence` (CUJ-7).
- R19: `tb stack` ganha a op `vault` (roda R16), escolhida como as outras, por argumento posicional (`tb stack verify guard vault`). Ela só lê e roda em paralelo como o `guard` (`isParallelOp`); ignora `--strict`, que continua sendo flag do `egm`. As listas de ops e as mensagens de erro em `internal/stack/stack.go` e `cmd/tb/stack.go`, e o schema da tool `tb_stack` em `internal/registry/parity.go`, incluem a op.

**Grafo (só com `TB_GRAPH=on`, ADR-011)**

- R20: `BuildOptions` ganha `VaultDir` (o vault do R1, vazio quando não resolve), preenchido por `graphOptionsFromConfig` e `graphBuildOptions`. Os indexadores por arquivo pulam os arquivos sob `VaultDir`, e um passo do build depois do walk (como o `resolveSoftMentions`) varre o vault inteiro uma vez pelo `internal/vault`, porque a resolução do R8 precisa de todas as notas. O passo roda quando `vault` está habilitado: `allowsIndexer` trata `vault` como habilitado sempre que `markdown` está em `Docs`, para que repositórios com `config.yaml` antigo (que lista `markdown, html, txt`) não percam as notas (G4), a menos que `vault.graph: false`; `vault` também entra no default de `Docs`.
- R21: o passo do vault **não** usa `readSource`: lê pelo `internal/vault` (R3). Symlink, FIFO e arquivo grande no vault nunca são lidos pelo grafo, então um repositório clonado não consegue, por um symlink versionado no vault, fazer o `graph.json` (e as tools `tb_graph_*`) carregar conteúdo de fora do repositório.
- R22: cada nota vira um nó `NodeNote` (novo) com ID `note:<caminho da nota>`, `Path` do arquivo e `Title` do R4, redigido (R15); `Source` e `Confidence` vêm do frontmatter (`manual` → 1.0, `extracted` → 0.9, `inferred` → o `confidence`). Headings (R6) viram `NodeSection` com ID `section:<arquivo>#<slug>`.
- R23: wikilink resolvido vira aresta `EdgeLinks` de `note:A` para `note:B`, ou para `section:<arquivo de B>#<slug>` com fragmento, `Source: "extracted"`, confiança 0.9. Links quebrados, ambíguos e de anexo não viram aresta. O corpo (sem frontmatter) continua gerando links Markdown, autolinks e menções como no `MarkdownIndexer`, com a nota como origem: `indexMarkupDoc` recebe o ID do nó de origem como parâmetro em vez de supor `file:`.
- R24: tags viram nós `NodeTag` (novos, `tag:<tag>`) com aresta `EdgeTagged` (nova) da nota para a tag.
- R25: `tb graph explain`, `tb graph path` e `tb graph search` alcançam nós `note:` e `tag:` pela resolução de ID exato que já existe (`resolveNodeIDs`); a mudança em `cmd/tb/graph.go` e `internal/registry/graph.go` é só listar os tipos novos de nó no help e nos schemas. Nenhum flag novo.

### Configuração

```yaml
vault:
  path: .toolbelt/vault        # existente
  sync_on_change: true         # existente (default atual true); reservado, sem efeito na Fase A
  max_note_bytes: 262144       # novo
  types: [fact, decision, context, log]  # novo
  graph: true                  # novo; false deixa as notas fora do grafo mesmo com markdown ligado
graph:
  docs: [markdown, vault, html, txt]     # vault novo no default; com só markdown, vault fica implícito (R20)
```

`VaultConfig.Validate` rejeita `path` absoluto, com `..` que escape da raiz, ou com quebra de linha, caractere de controle ou caractere de glob (`*`, `?`, `[`, `]`, `!`, `#` no começo), já que ele é gravado no `.git/info/exclude`; `max_note_bytes` ≤ 0 ou acima de 4 MiB; e `types` vazio, com valores repetidos ou fora de `^[a-z][a-z0-9_-]*$`.

## Modelo de ameaças

| Ameaça | Mitigação |
|--------|-----------|
| Nota escrita por agente ou vinda de repositório clonado instrui outro agente (injeção via MCP) | R18 (todo dado de nota dentro de `data`), R15. Risco residual: o agente ainda lê o texto e pode obedecê-lo. |
| Combinação "dado privado + conteúdo não confiável + saída externa" (ADR-010) | O tb não tem saída externa (local-first). O `tb_vault_*` entrega conteúdo não confiável ao agente, e é o harness do agente que decide se ele tem saída externa. Fica registrado como risco do harness, não do tb. |
| Segredo colado numa nota chega a um prompt | R10 (redação na escrita), R15 (redação na leitura, antes do recorte), R13 (busca sobre o texto redigido, sem sondagem pela contagem de acertos). Risco residual: segredos fora dos padrões do R15. |
| Segredo no título vira nome de arquivo | R10 (o slug sai do título redigido) |
| Título com quebra de linha forja frontmatter (`source: manual`) | R10 (caracteres de controle recusados, frontmatter por `yaml.Marshal`) |
| `vault.path` ou `--dir` apontando para fora; symlink em componente intermediário; padrão injetado no `.git/info/exclude` | R1 (`EvalSymlinks` da raiz do vault), R10 (`Lstat` por componente, `O_NOFOLLOW`), `VaultConfig.Validate` |
| Symlink versionado no vault para `~/.ssh/...` lido pelo grafo e devolvido por `tb_graph_*` | R21 |
| Nota gigante ou FIFO trava o scan, o `tb graph index` ou o `tb stack`, inclusive com troca entre o `Lstat` e a abertura | R3 (`O_NONBLOCK` + `fstat`), R21 |
| `tb vault new` sobrescreve nota humana | R10 (`O_EXCL`) |
| Nota de agente se passa por conhecimento humano (`source: manual`, confiança 1.0) | R18 (MCP grava sempre `inferred`). Risco residual: um agente com acesso a arquivos ou à CLI pode escrever `manual` à mão; a proveniência é declarativa, não assinada. |
| Títulos e tags de notas chegam a agentes pelas `tb_graph_*`, fora de um objeto `data` | R22 (títulos redigidos). Risco residual: as tools do grafo devolvem títulos de nó como campos simples, então um título com injeção ainda chega ao agente. |
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
- **Exportar `slugify` do grafo e importar o grafo no vault.** Rejeitada: ciclo de import com o passo do vault. Daí o pacote folha `mdparse`.
- **Acrescentar os padrões de segredo novos ao `mem`.** Rejeitada na Fase A: o `mem.secretPatterns` também alimenta o `RedactInvariant` e o `tb egm`, então mudaria a saída deles (G6). O vault mantém a própria lista em cima de `mem.SecretPatterns()`.

## Questões em aberto

- Q1 (resolvida): as divergências da ADR-006 (`.tb/vault/`, índice SQLite, sync, `tb md`) e da ADR-003 (SQLite+FTS5 como índice derivado) estão registradas na [ADR-016](../adr/016-vault-fase-a.md), que substitui as duas em parte na Fase A.
- Q2: wikilink para invariante (`[[inv:<id>]]`) resolvendo para `invariants.yaml`? Proposta: Fase B.
- Q3: `tb review` e `tb drift` deveriam ler notas `type: decision` ligadas ao ticket? Fora da Fase A. Notas nunca são fontes do `tb egm`: o egm exclui `.toolbelt/**` das fontes, o que também mantém o texto das notas fora dos pacotes dos revisores.
- Q4: a fixture dourada (G1) precisa ser conferida uma vez no Obsidian real antes de congelar o R8; até lá, as regras de caminho parcial e de anexo do R8 são o comportamento esperado, não um verificado.
- Q5: o walk do grafo fora do vault tem a mesma fraqueza do R21 (`readSource` segue symlinks e lê FIFOs). Vale um fix separado, com o mesmo leitor seguro, para todos os indexadores.
- Adiado para a Fase B (cortado nesta rodada para manter a Fase A pequena): o campo `refs` do frontmatter com `bad_ref`/`ref_missing` e o passo do `Store.Impact` para notas, `orphan_note`, `check --strict`.

## Plano de Implementação Detalhado

Quatro fatias, cada uma mergeável sozinha, com seus testes: 1 `mdparse` (nenhuma mudança visível); 2 vault só leitura e `tb stack vault`; 3 escrita, busca, MCP e versionamento; 4 grafo.

| Arquivo | Ação | Motivo |
|---------|------|--------|
| `internal/mdparse/slug.go`, `internal/mdparse/headings.go`, `internal/mdparse/frontmatter.go`, `internal/mdparse/wikilink.go`, `internal/mdparse/normalize.go` | Create | Fatia 1: Pacote folha (R4, R6, R7, R13) |
| `internal/mdparse/mdparse_test.go` | Create | Fatia 1: Casos de parser do plano de verificação |
| `internal/graph/index_docs.go` | Modify | Fatia 1: Usar `mdparse.Slugify` e headings com cerca (R6) |
| `internal/mem/validate.go` | Modify | Fatia 1: Reusar `mdparse.NormalizeForSearch` |
| `internal/config/config.go`, `internal/config/config_test.go` | Modify | Fatia 2: `max_note_bytes`, `types`, `graph`, `VaultConfig.Validate` (R0, R1, Configuração) |
| `internal/vault/root.go`, `internal/vault/scan.go`, `internal/vault/read.go` | Create | Fatia 2: Raiz e config a partir da raiz, walk e leitura seguros (R0, R1, R3) |
| `internal/vault/note.go`, `internal/vault/resolve.go`, `internal/vault/redact.go` | Create | Fatia 2: Nota, defaults, classificação, resolução, redação do vault (R4–R9, R15) |
| `internal/vault/vault.go`, `internal/vault/check.go` | Create | Fatia 2: `List`, `Show`, `Links`, `Backlinks`, `Check` (R11, R12, R14, R16) |
| `internal/vault/vault_test.go` | Create | Fatia 2: Testes de resolução, frontmatter, segurança e da fixture dourada |
| `cmd/tb/vault.go`, `cmd/tb/vault_test.go` | Create | Fatia 2: `list`, `show`, `links`, `backlinks`, `check` (R17) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | Fatia 2: Op `vault` (R19) |
| `internal/vault/new.go`, `internal/vault/search.go` | Create | Fatia 3: `New`, `Search` (R10, R13) |
| `internal/registry/vault.go` | Create | Fatia 3: Tools `tb_vault_*` (R18) |
| `internal/registry/parity.go`, `internal/registry/parity_test.go`, `internal/registry/annotations.go` | Modify | Fatia 3: Registro, paridade, op `vault` no schema de `tb_stack` (R18, R19) |
| `internal/config/exclude.go`, `internal/config/exclude_test.go` | Modify | Fatia 3: Padrões extras, tirar `vault/`, marcador de início órfão (R2) |
| `cmd/tb/main.go`, `cmd/tb/init_exclude_test.go` | Modify | Fatia 3: `init` carrega config, cria vault customizado, loga padrões, `vault_gitignored` (R2, CUJ-6) |
| `.gitignore` | Modify | Fatia 3: Parar de ignorar `.toolbelt/vault/` neste repositório (G5) |
| `cmd/tb/json_snake_case_test.go` | Modify | Fatia 3: Amostras das saídas `tb vault` |
| `docs/cli.md`, `docs/arquitetura.md`, `README.md` (+ espelhos em `docs/pt-BR/`) | Modify | Fatia 3: Cada doc com o seu espelho (o teste de tradução das docs exige). Trocar `.tb/vault` por `.toolbelt/vault`; marcar SQLite, `tb md` e sync como planejados; documentar `tb vault` |
| `CHANGELOG.md` | Modify | Fatia 3: Entrada não lançada |
| `internal/graph/indexer.go` | Modify | Fatia 4: `BuildOptions.VaultDir`, passo do vault depois do walk, `vault` implícito (R20, R21) |
| `internal/graph/index_vault.go`, `internal/graph/index_vault_test.go` | Create | Fatia 4: Notas, seções, arestas de wikilink e de tag (R22–R24) |
| `internal/graph/index_docs.go` | Modify | Fatia 4: `indexMarkupDoc` com parâmetro de ID de origem; pular `VaultDir` (R20, R23) |
| `internal/graph/graph.go`, `internal/graph/graph_test.go` | Modify | Fatia 4: `NodeNote`, `NodeTag`, `EdgeTagged` (R22, R24) |
| `internal/registry/graph.go`, `cmd/tb/graph.go` | Modify | Fatia 4: `VaultDir` nas opções de build; tipos novos de nó no help e nos schemas (R20, R25) |

### Plano de verificação

- **Parser (`mdparse`):** um teste por forma do glossário: `[[a]]`, `[[a|b]]`, `[[a\|b]]` (em tabela), `[[a#h]]`, `[[a#^x]]`, `[[#h]]`, `![[a]]`, `[[pasta/a]]`, `[[a.md]]`, `[[ a ]]`, `[[A]]` contra `a.md`; ignorados: dentro de ` ``` `, de `~~~`, de código inline com uma e duas crases, no frontmatter; descartados: `[[]]`, `[[a]` + quebra de linha. Headings dentro de cerca e comentários YAML não viram heading. CRLF: linhas e frontmatter corretos.
- **Resolução:** nome único; caminho completo; caminho parcial (`[[b/a]]` contra `x/b/a.md`); dois `a.md` em pastas diferentes (`ambiguous_link`); alias nunca resolve; anexo encontrado e ausente (aviso `missing_attachment`); fragmento existente e inexistente.
- **Fixture dourada (G1):** vault em `internal/vault/testdata/obsidian/` com o resultado esperado de cada link em `expected.json`, conferido uma vez no Obsidian real (registro em `docs/campo/dogfood.md`); o teste compara o tb com o `expected.json`.
- **Frontmatter:** cada linha da tabela de classificação do R4.
- **Segurança:** symlink para fora do vault (scan e grafo); diretório intermediário symlink em `--dir`; FIFO `x.md` no vault (scan, `tb graph index` e `tb stack vault` terminam); nota acima do limite; `vault.path: ../x`, com quebra de linha e com caractere de glob; `vault.path` inexistente (`vault_not_found`, exit 1); título com quebra de linha (`bad_title`) e com segredo (nome de arquivo redigido); segredo no `--body` (gravado redigido); segredo numa nota editada à mão, inclusive cortado pela janela do trecho (saída redigida em `show`, `search` e MCP) e um `search` pelo prefixo dele (sem acerto); `tb_vault_new` gravando `inferred` mesmo se o agente tentar `source`; `cwd` do MCP diferente do do servidor.
- **Exclude:** `init` em repositório com a seção antiga troca `vault/` por `vault/.obsidian/` (checagem por linha inteira, não substring, porque `.toolbelt/vault/.obsidian/` contém `.toolbelt/vault/`); arquivo com marcador de início órfão preserva as linhas do usuário, descarta o marcador órfão e as linhas antigas do tb, e um segundo `init` não muda nada; `vault.path` customizado gera o padrão e o diretório certos; `vault_gitignored` quando o `.gitignore` ignora o vault.
- **Grafo:** com `TB_GRAPH=on` e um `config.yaml` antigo (`docs: [markdown, html, txt]`), cada nota gera um único nó `note:` e nenhum `file:`; arestas de wikilink, fragmento e tag; `vault.graph: false` deixa as notas fora; títulos de nota redigidos; `explain note:` e `path` entre duas notas; com o grafo desligado, `tb vault` funciona igual.
- **Regressão:** testes de `tb mem`, `tb egm`, `tb graph`, `json_snake_case`, paridade do registry e espelho de docs passam.
- **Desempenho:** benchmark Go com 2 000 notas geradas (NFR).
