[🇺🇸 English](../../pesquisa/mcp-sinais-m8ven.md) | 🇧🇷 Português

# Servidor MCP do tb contra os sinais do M8ven

> **Data da revisão**: 2026-11-18
> **Referência**: [m8ven/is-it-legit-mcp](https://github.com/m8ven/is-it-legit-mcp) no commit `61dd093`, [M8ven Verified](https://m8ven.ai/verified/how-we-verify) e o [relatório público](https://m8ven.ai/mcp/is-it-legit-mcp) do próprio servidor deles
> **Alvo**: `tb mcp` ([internal/mcp/server.go](../../../internal/mcp/server.go) e [internal/registry/](../../../internal/registry/))

## Por que o M8ven serve de referência

O M8ven dá nota (A–F) a servidores MCP. A metodologia pública não lista os critérios nem a fórmula da nota, mas o relatório deles e o próprio repositório mostram em que se apoia:

| Sinal | O que o M8ven olha |
|---|---|
| **Code Verified** | O código-fonte faz o que as ferramentas declaram. Na prática, as anotações MCP batem com o efeito real do handler. |
| **Sandbox Verified** | O servidor rodou num ambiente isolado e o comportamento foi observado. |
| **Live Monitored** | Reverificado a cada push, via GitHub App. |
| Exfiltração e ofuscação | O servidor não manda credenciais para fora, não lê arquivos sensíveis e não tem código ofuscado. |
| Dependências | Vulnerabilidades conhecidas nas dependências de produção. O servidor deles leva rebaixamento por 5 dependências com severidade alta. |
| Open source | Licença e README presentes. |
| Publisher | Prova de controle do repositório ou do host. O repositório deles traz `.well-known/m8ven-publisher.txt`. |
| Manutenção | Issues fechadas, releases e retenção de contribuidores nos últimos 90 dias. |

O padrão mais útil do repositório é o README: cada ferramenta tem as quatro anotações (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`), cada uma **justificada pelo que o handler faz de fato**. Em vez de "é uma consulta", a justificativa diz, por exemplo, que `check_brand` faz upsert de cache com `onConflict` e por isso é destrutiva. Esse rigor é o que o selo Code Verified mede.

## Relatório atual do tb no M8ven

[Listagem do tb](https://m8ven.ai/mcp/leonardo-moraes360-toolbelt-fa03ue), consultada em 2026-11-18:

- **Nota C, status Limited view.** O código foi avaliado no commit `531c9bc` (15/11), mas a superfície de ferramentas não foi lida. A análise estática de Go tem cobertura parcial.
- **Passam:** sem exfiltração de credenciais, acesso a arquivo sensível ou ofuscação; open source com licença e README.
- **Faltam:** listagem não reivindicada (sem Verified Publisher), Live Monitored desconectado, Sandbox Verified ausente.
- A trilha completa (cada checagem, deduções, advisories) só aparece para publisher verificado.

O "passa" de exfiltração vem de uma análise estática parcial. O achado 2 abaixo continua valendo: o vetor é o modelo pedir o segredo em tempo de execução, e não código que envia o segredo para fora. Reivindicar a listagem e ligar o GitHub App tira o status Limited view e abre a trilha completa.

## Achados

### Alta

1. **Corrigido: nenhuma ferramenta declarava anotações.** `registry.Tool` só tem `Name`, `Description` e `InputSchema` ([registry.go:9](../../../internal/registry/registry.go:9)), e o `tools/list` não emite `annotations` ([server.go:142](../../../internal/mcp/server.go:142)). Sem as dicas, o cliente trata `tb_ticket_list` e `tb_run` do mesmo jeito: não dá para aprovar consultas automaticamente nem marcar as ferramentas perigosas. Esse é exatamente o sinal Code Verified.
   *Correção:* adicionar um campo `Annotations` em `Tool`, emitir no `tools/list` e preencher conforme a tabela abaixo. Um teste deve garantir que toda ferramenta registrada declara as quatro dicas.
   *Status:* corrigido. [annotations.go](../../../internal/registry/annotations.go) guarda a tabela, o `Register` a aplica, o `tools/list` emite as quatro dicas explicitamente e o `TestAllToolsAnnotated` falha se alguma ferramenta ficar sem entrada.

2. **Corrigido: o segredo era opt-in do modelo, não do humano.** `tb_run`, `tb_verify_run` e `tb_stack` aceitam `pass_env` como argumento da chamada ([builtin.go:60](../../../internal/registry/builtin.go:60) e [parity.go:185](../../../internal/registry/parity.go:185)). A allowlist de ambiente é boa, mas qualquer agente com prompt injection pode pedir `pass_env:["GITHUB_TOKEN"]` e imprimir o valor, porque `tb_run` aceita qualquer `sh -c`. Pela trifeta letal (dado privado + conteúdo não confiável + saída externa), isso é o achado "credential exfiltration" do M8ven.
   *Correção:* aceitar `pass_env` só da configuração (`.toolbelt` ou flag do `tb mcp`) e não do argumento da ferramenta, ou exigir que a chave já esteja liberada na configuração.
   *Status:* corrigido. Via MCP, [passenv.go](../../../internal/registry/passenv.go) recusa qualquer chave de `pass_env` que não esteja em `env.pass` (resultado `isError` com o nome da chave), em `tb_run`, `tb_verify_run` e `tb_stack`. A flag `--pass-env` da CLI não mudou, porque quem a digita é um humano. Coberto por `TestToolPassEnvRefusedWithoutConfigOptIn`. Risco residual, agora fechado pelo achado 8: um agente podia gravar `env.pass` no `.toolbelt/config.yaml` do repositório e pedir de novo. O `pass_env` agora só confia no `env.pass` da config global ou de `-c`.

### Média

3. **Corrigido: as consultas ao grafo escreviam.** `tb_graph_search`, `callers`, `deps`, `impact`, `explain` e `path` chamam `openGraphStoreFresh` ([graph.go:121](../../../internal/registry/graph.go:121)), que reconstrói o índice quando ele está obsoleto e `graph.index_on_change` está ligado (o padrão) ([fingerprint.go:100](../../../internal/graph/fingerprint.go:100)). O nome sugere leitura, mas a chamada pode escrever em disco. O M8ven trata o mesmo caso (upsert de cache em `brand_signals`) como `readOnlyHint:false`.
   *Correção:* anotar `readOnlyHint:false, destructiveHint:false, idempotentHint:true`, ou passar `indexOnChange=false` no caminho MCP e devolver `stale:true`.
   *Status:* corrigido com a segunda opção. Via MCP, as consultas abrem o índice com `openGraphStoreReadOnly` ([graph.go](../../../internal/registry/graph.go)), que só confere o fingerprint. Índice obsoleto devolve `stale: true` e um `hint` para chamar `tb_graph_index`, a única ferramenta de grafo que escreve. As consultas agora são anotadas com `readOnlyHint:true`. A CLI continua respeitando `graph.index_on_change`. Coberto por `TestGraphQueriesNeverWriteIndex` e `TestGraphQueryWithoutIndexCreatesNothing`. Custo: depois de uma edição, um agente que ignore o `stale` recebe resultados do índice antigo até chamar `tb_graph_index`.

4. **Corrigido: a versão do protocolo estava congelada em `2024-11-05`** ([server.go:16](../../../internal/mcp/server.go:16)), sem negociação. As anotações de ferramenta só entraram na spec em `2025-03-26`, e o [cli.md](../cli.md) e as fontes já citam alinhamento com a spec `2026-07-28` stateless. Um cliente estrito pode ignorar `annotations` numa sessão que se anuncia como 2024-11-05.
   *Correção:* ler `params.protocolVersion` do `initialize` e aplicar a regra da spec: se a versão for suportada, responder com a mesma; se não, responder com a mais recente suportada.
   *Status:* corrigido em [server.go](../../../internal/mcp/server.go). O servidor suporta `2025-11-25` (a mais recente, que ele oferece), `2025-06-18` e `2024-11-05`. Versão ausente ou malformada recebe a mais recente. Coberto por `TestInitializeNegotiatesProtocolVersion`. Duas revisões ficam de fora de propósito:
   - `2025-03-26` exige batching JSON-RPC, que o servidor não implementa. Um cliente nessa versão recebe a oferta de `2025-11-25`.
   - `2026-07-28` troca o `initialize` por `server/discover` e `_meta` por requisição, e exige `resultType`, `ttlMs` e `cacheScope` nos resultados. É outro ciclo de vida, ainda pendente em [arquitetura.md](../arquitetura.md).

5. **Corrigido: a ordem do `tools/list` não era determinística.** `Registry.List` percorre um map ([registry.go:61](../../../internal/registry/registry.go:61)). A spec 2026-07-28 pede listas cacheáveis com ordem determinística, e a ordem instável também quebra o cache de prompt do cliente.
   *Correção:* `sort.Slice` por nome.
   *Status:* corrigido em `Registry.List`, coberto por `TestListSortedByName` e `TestToolsListEmitsAnnotationsInOrder`.

### Baixa

6. **Publisher.** O tb já está listado, mas a listagem não foi reivindicada e não há `.well-known/m8ven-publisher.txt`. Reivindicar dá o selo Verified Publisher e a trilha completa.
7. **Supply chain.** `govulncheck` (CI, [ci.yml:61](../../../.github/workflows/ci.yml:61)) e `gosec` (golangci) já cobrem o sinal de dependências. Esta revisão não rodou `govulncheck` localmente porque a ferramenta não está instalada.
8. **Corrigido: Sandbox Verified.** O `tb mcp` rodava fora do sandbox do próprio tb: `tb_run` executava sem Landlock nem bwrap. O sinal de sandbox do M8ven observaria escrita fora do repositório.
   *Status:* corrigido em [runsandbox.go](../../../internal/registry/runsandbox.go). O `tb_run` roda sob Landlock: escreve só na raiz do repositório, no próprio `TMPDIR` e nos caches de toolchain; lê os diretórios do sistema, o `PATH`, o `GOROOT` e os caminhos `sandbox.read`/`write` da config global. `$HOME`, `/tmp` e outros repositórios ficam fechados, e `cwd` fora do repositório é recusado. Sem Landlock, falha fechado, a menos que `sandbox.allow_unsandboxed`. Como o repositório é gravável, os ajustes de segurança (`env.pass`, `sandbox.*`) agora vêm só da config global ou de `-c` (`config.LoadTrusted`), o que fecha o risco residual do achado 2. Coberto pelos testes `TestTBRun*` em [runsandbox_test.go](../../../internal/registry/runsandbox_test.go). Continua aberto: a rede não é restringida, e `tb_verify_run`/`tb_stack` seguem sem sandbox e ainda leem `env.pass` da config do repositório.

### Já atende

- Licença, README bilíngue, SECURITY e releases (sinais de open source e manutenção).
- Falha de ferramenta volta como `isError:true` no resultado, e não como erro JSON-RPC ([registry.go:81](../../../internal/registry/registry.go:81)).
- `tb_bus_recv` avisa que o conteúdo de outros agentes é dado não confiável.
- `tb_merge` faz dry-run por padrão e exige `apply:true` e `allow_sensitive` para caminhos sensíveis.
- Sem rede no servidor: as únicas saídas são processos filhos (`tb_run`, verify, review).

## Anotações (implementadas)

As justificativas seguem o formato do M8ven: o efeito real do handler, não o nome da ferramenta.

| Ferramenta | readOnly | destructive | idempotent | openWorld | Justificativa |
|---|---|---|---|---|---|
| `tb_version`, `tb_echo`, `tb_env` | true | false | true | false | Só leem o processo. |
| `tb_ticket_list`, `tb_ticket_show`, `tb_mem_list`, `tb_mem_show`, `tb_mem_search`, `tb_mem_check`, `tb_router_status`, `tb_watchdog_alerts` | true | false | true | false | Leem `.toolbelt/` sem escrever. |
| `tb_graph_search`, `callers`, `deps`, `impact`, `explain`, `path` | true | false | true | false | Leem o índice e nunca o reconstroem via MCP. Índice obsoleto devolve `stale: true` (achado 3). |
| `tb_graph_index` | false | true | true | false | Sobrescreve o índice. |
| `tb_ticket_create`, `tb_mem_add` | false | false | false | false | Insert. Cada chamada cria um item novo. |
| `tb_mem_edit`, `tb_mem_retire` | false | true | true | false | Sobrescrevem campos de uma invariante (retire é soft, mas altera o estado). |
| `tb_mem_init` | false | false | true | false | Cria os padrões só se faltarem (`mem.EnsureDefaultAntiCheat`). |
| `tb_board` | false | true | true | false | `pause`/`resume`/`drain` alteram o estado do router. `list` só lê, mas a anotação vale para a ferramenta inteira. |
| `tb_bus_join`, `tb_bus_send`, `tb_bus_claim`, `tb_bus_complete` | false | false | false | true | Escrevem no barramento lido por outros agentes (mundo aberto entre agentes). |
| `tb_bus_recv` | false | false | false | true | Avança o cursor. Repetir a chamada devolve outro lote. |
| `tb_merge` | false | true | false | false | Com `apply:true`, altera refs. O dry-run é o padrão, mas a anotação cobre o pior caso. |
| `tb_review`, `tb_drift`, `tb_guard_check` | true | false | true | false | Analisam o diff (`review` só roda `git patch-id`; não escreve). |
| `tb_verify_run`, `tb_stack` | false | true | false | true | Executam comandos do usuário com env filtrado. |
| `tb_run` | false | true | false | true | `sh -c` sob Landlock: pode apagar ou sobrescrever qualquer coisa no repositório, e a rede fica aberta. |

Os handlers de `tb_mem_init`, `tb_review` e `tb_stack` foram lidos para confirmar estes valores. O `TestAllToolsAnnotated` falha se alguma ferramenta nova for registrada sem entrada.
