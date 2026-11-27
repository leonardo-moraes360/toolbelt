[🇺🇸 English](../../pesquisa/ontologia.md) | 🇧🇷 Português

# Ontologia para agentes: o que se aplica ao tb

> **Data da revisão**: 2026-11-27
> **Referência**: o explainer "What Is Ontology in AI?" (transcrição de vídeo colada pelo mantenedor; autor e publicação não identificados), que cita a Ontology da Palantir, a Genie Ontology da Databricks e o [GraphRAG](https://arxiv.org/abs/2404.16130) da Microsoft (já em [fontes-primarias.md](fontes-primarias.md))
> **Alvo**: `tb graph` ([internal/graph/](../../../internal/graph/), [ADR-011](../adr/011-grafo-sob-demanda.md), [ADR-014](../adr/014-graphify-reimplementacao.md)) e o vault ([internal/vault/](../../../internal/vault/), [ADR-016](../adr/016-vault-fase-a.md))

## O argumento

O texto é um explainer geral, sem dados nem fonte primária própria. O argumento é uma sequência de camadas, cada uma tapando um buraco da anterior:

1. Colar tudo no prompt, depois RAG: o modelo recebe os fatos certos, mas como pedaços soltos.
2. Grafos de conhecimento: os fatos viram coisas e relações, mas um grafo diz o que está ligado, não o que significa.
3. MCP: o agente acha as tools ("os números de telefone"), mas não como a organização funciona ("o organograma").
4. Skills: o agente aprende procedimentos, mas sobre termos que cada sistema nomeia de um jeito.
5. Ontologia: um modelo compartilhado do que existe, o que significa, como se relaciona, qual fonte é autoritativa, que ações existem e quem pode executá-las.

O exemplo da lacuna é um conceito com vários nomes: "client", "account", `customer_id`, "billing entity" e "organisation" são a mesma coisa em cinco sistemas, e nada avisa o agente disso.

## Onde o tb está

- **O grafo é estrutural.** Os tipos de nó são `file`, `function`, `package`, `class`, `section`, `component`, `hook`, `route`, `playbook`, mais `note` e `tag` vindos do vault. Os tipos de aresta são `imports`, `calls`, `defines`, `links`, `mentions`, `renders`, `template`, `mirror` e `tagged`. Não há nó para um conceito do domínio.
- **Docs já apontam para código, sem tipo.** Quando um doc cita um identificador entre backticks, o indexador cria uma aresta `mentions` do doc para aquele nó de código, com uma confiança ([`docMentionEdges`](../../../internal/graph/index_docs.go)). Uma nota sobre "Pedido" que cita `` `OrderDTO` `` já está ligada a ele, mas a aresta não diz que a nota *define* o conceito e o código o *implementa*.
- **O vault conhece sinônimos, o grafo não.** As notas aceitam `aliases` no frontmatter ([note.go](../../../internal/vault/note.go)), e o `tb vault search` casa com eles. O `tb graph search` (BM25 sobre o código) não os usa, então uma busca por "customer" não acha `client_id`.
- **A camada de ações e permissões existe em outro lugar.** As anotações das tools MCP (da [revisão M8ven](mcp-sinais-m8ven.md)) e as capabilities do registry já dizem o que cada tool faz e o que pode tocar.

## Propostas, por valor

### 1. Notas de conceito como nós tipados no grafo

As notas do vault já têm um campo `type`, checado contra `vault.types` (padrão `fact`, `decision`, `context`, `log`). Acrescentando `concept` a essa lista, uma nota com `type: concept` vira um nó `concept` no grafo. As menções em backtick a código viram arestas `represents` (conceito → código) em vez de `mentions` simples. Aí o `tb graph explain <conceito>` devolve, numa chamada, os tipos, tabelas, rotas e testes que formam aquele conceito no repositório.

Isso reusa o que existe: o campo `type` do vault, o extrator de menções e sua confiança. As únicas partes novas são um tipo no vault, um tipo de nó no grafo e um tipo de aresta. Atende direto o objetivo "economizar tokens" do tb, porque o agente deixa de fazer grep por cada grafia de um termo do domínio.

**Prioridade**: média · **Módulo**: graph · vault

### 2. Aliases na busca de código

Com `TB_GRAPH=on` e o vault indexado, o `tb graph search` expande a consulta com os `aliases` de toda nota de conceito cujo título ou alias case com a consulta. O resultado diz qual alias casou, para o agente distinguir um acerto expandido de um literal.

**Prioridade**: média (mesma mudança do item 1) · **Módulo**: graph

### 3. Fonte autoritativa por conceito

Um campo opcional `authoritative:` na nota de conceito nomeia o arquivo, a tabela ou o sistema que vale quando duas fontes divergem. Combina com a regra do tb de que o vault é a fonte da verdade e o índice é derivado, e pode alimentar depois as checagens de drift do egm.

**Prioridade**: baixa · **Módulo**: vault · egm

## Não vale trazer

- **Um motor de ontologia** (OWL, RDF, reasoners). O tb é um CLI local; notas markdown com frontmatter mais o grafo que já existe cobrem o que um agente precisa num repositório.
- **Ações e permissões por tipo de objeto**, o lado Palantir do texto. As anotações MCP e as capabilities do registry já fazem isso para as tools do tb, e o tb não modela objetos de negócio.
- **Databricks Genie e Palantir Ontology** como produtos: plataformas corporativas de dados, sem relação com um CLI local.

## Ordem sugerida

Itens 1 e 2 numa mudança só, depois da v1.0.0; depois o item 3, se as notas de conceito forem usadas.
