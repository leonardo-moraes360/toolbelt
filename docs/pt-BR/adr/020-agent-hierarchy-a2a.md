[🇺🇸 English](../../adr/020-agent-hierarchy-a2a.md) | 🇧🇷 Português

# ADR-020: hierarquia de agentes como árvores de delegação no barramento local; o A2A fica como formato de envelope, sem rede no `tb`

## Status

aceito (2026-11-22). Substitui em parte o [ADR-002](002-mcp-a2a-protocolos.md): a seção "A2A para Agentes", o socket Unix do `tb serve` e o transporte HTTP/2. Confirma a rejeição de "A2A sobre HTTP como está" no [ADR-013](013-barramento-multi-vendor.md) e o abandono do `tb md` no [ADR-016](016-vault-fase-a.md) §5. Spec: [`docs/pt-BR/specs/agent-hierarchy.md`](../specs/agent-hierarchy.md).

**Nota de implementação.** A Fase A está implementada (`internal/bus/hierarchy.go`, `internal/bus/tree.go`, `tb bus send --parent`, `tb bus tree`, `parent` no `tb_bus_send` do MCP, schema v4 do store). As três questões abertas da spec foram resolvidas pelo usuário: construir a Fase A; `complete` com filhos abertos só avisa; os limites (profundidade 3, 8 filhos abertos) são constantes. O abandono do `tb agent` está confirmado.

## Contexto

O roadmap do README carrega uma linha desde a Fase 2: "Hierarquia de agentes / A2A / `tb md` (posterior)". Ela junta três coisas que estão em estados diferentes.

**A2A.** O ADR-002 (aceito) escolheu A2A para o tráfego agente ↔ agente, com delegação Planner → Supervisor → Worker sobre um socket Unix do `tb serve` e HTTP/2 "para remoto" depois. Nada disso foi construído. O que existe é o `tb bus` ([ADR-013](013-barramento-multi-vendor.md), [bus-protocolo.md](../bus-protocolo.md)): um envelope JSON-RPC 2.0 (`bus/0.1`) com primitivas inspiradas no A2A sobre um store SQLite. Não há socket, servidor HTTP nem cliente A2A em lugar nenhum do código. O ADR-013 rejeitou "A2A sobre HTTP como está" porque agentes de fornecedores não expõem servidores, e manteve o envelope no formato A2A "para facilitar uma ponte futura".

**Hierarquia.** A delegação que existe hoje:

- `tb router run` ([ADR-009](009-router-e-workers.md)) é um escalonador determinístico: um ticket por worker, workers locais no sandbox ou workers do barramento pelos slots do B4 ([bus-router.md](../bus-router.md)).
- Os papéis do barramento (`router`, `supervisor`, `worker`, `reviewer`) são fixados pelo convite que o humano emite. Só `router` e `supervisor` podem enviar `task.assign`; só o dono do claim pode dar `complete`; leases, heartbeats e o watchdog recuperam donos perdidos.
- Um agente supervisor já pode dar claim numa thread e enviar `task.assign` para subtrabalho. Mas o barramento não sabe que a subthread pertence à primeira. `thread` e `causation_id` são livres, então nenhum comando mostra a árvore, limita o fan-out ou avisa o supervisor de que um filho ainda está aberto quando ele completa a própria thread.

O `docs/arquitetura.md` §8 e o `docs/cli.md` (`tb agent spawn --parent`, `approve`, `reject`) ainda descreviam uma árvore planner/supervisor/worker com orçamento de tokens por nível como se fosse o plano.

**`tb md`.** O ADR-016 tirou a TUI da Fase A e disse que o README e o `docs/arquitetura.md` deviam parar de descrevê-la. Os dois ainda descreviam, assim como o `docs/cli.md` e o `docs/identidade-visual.md`.

Restrições (invariantes do projeto, `.toolbelt/invariants.yaml`):

- O `tb` é 100% local e não abre rede; rede só no processo filho que o humano configurou.
- A espera no barramento não usa LLM.
- O MCP do barramento nunca cria canais nem convites; a identidade fica presa à conexão e o broker define o `from`.
- O sandbox é fail-closed; o harness nunca usa `bypassPermissions`.
- Implementador e avaliador são separados.

## Decisão

### 1. A2A: o envelope local `bus/0.1` é a resposta; o `tb` não traz transporte A2A

- O `tb` não contém servidor, cliente, listener HTTP ou socket A2A, nem adapter `a2a` para o `tb bus run`. O envelope `bus/0.1` continua no formato A2A. A tabela abaixo é um guia **não normativo** para quem mapear os dois; ela não acrescenta protocolo.

  | Conceito A2A | `bus/0.1` |
  |---|---|
  | Agent Card | Agent Card no `join` (informativo; o papel vem do convite) |
  | Task | uma thread com `task.assign` e um claim |
  | Estados da task (working, completed, canceled, ...) | com claim, completada (`task.result`), revogada |
  | Message | `query` / `reply` / `task.status` |
  | Artifact | referências no payload (caminhos, SHAs, IDs de ticket), nunca blobs |

- **Uma ponte é um agente comum do barramento.** Se o humano quiser que um agente A2A remoto participe, a ponte é um programa separado que o humano escreve ou instala, roda fora do `tb` e admite com convite e papel próprios, como qualquer outro agente. A rede fica com ela, não com o `tb`. Ela é não confiável como qualquer agente, e o papel limita o que pode fazer: uma ponte `worker` não atribui trabalho, então uma parte remota não empurra tarefas para o canal a menos que o humano tenha feito da ponte um `supervisor`.
- O adapter `exec` do `tb bus run` já deixa um script escrito pelo humano chamar a rede a cada despertar (um webhook, por exemplo). Esse é todo o suporte a rede em volta do barramento.

Por que não uma ponte ou adapter embutido: os adapters do harness são chamadas curtas de despertar, enquanto um servidor A2A escuta o tempo todo. Colocar um sob o `tb` faria do `tb` o pai de um listener de rede. A2A de entrada também deixaria qualquer cliente remoto que o alcance pôr trabalho na máquina local por meio de um único convite, e o modelo de convites dá consentimento por agente.

### 2. Hierarquia: router + papéis do barramento é a hierarquia; a Fase A acrescenta árvores de delegação no barramento

A delegação já tem um nível e funciona: o router ou um supervisor atribui, os workers dão claim e completam. Não há papel de planner nem processo novo. O humano (ou um agente supervisor que o humano admitiu) planeja, e o router escalona de forma determinística.

A Fase A (um PR, [spec](../specs/agent-hierarchy.md)) torna real o segundo nível no barramento, dentro das regras atuais do broker:

1. **`parent` no `task.assign`.** Um campo opcional do envelope que nomeia a thread pai. O broker só o aceita quando o remetente é o `router` ou tem o claim vivo da thread pai, e só no primeiro `task.assign` de uma thread nova. O pai de uma thread nunca muda, então não se formam ciclos.
2. **Limites estruturais em vez de orçamento de tokens.** No máximo 3 níveis abaixo de uma raiz e no máximo 8 filhos abertos por pai, os dois checados pelo broker. O `tb` não mede os tokens de um agente de fornecedor, então um orçamento de tokens repassado por ele seria um número que ninguém aplica. Profundidade e fan-out são o que o broker consegue aplicar.
3. **`tb bus tree <canal>`.** Uma visão de CLI só leitura e só do humano (como o `tb bus log`) de cada árvore: thread, pai, estado (atribuída, com claim, completada, revogada, liberada), dono e época. Não mostra payload.
4. **Filhos abertos são informados, não bloqueiam.** `complete` num pai com filhos abertos funciona. O resultado avisa, e o `tb bus tree` marca.
5. **CLI e MCP.** `tb bus send --parent T` e um argumento `parent` no `tb_bus_send`. Nenhuma ferramenta MCP nova cria canais ou convites, e `tree` é só CLI.

O schema do store vai de v3 para v4 (uma tabela nova, upgrade de mão única, mesmas regras de v1 → v2 e v2 → v3).

### 3. `tb md` e `tb agent` saem

- O `tb md` continua fora (ADR-016 §5): qualquer editor, Obsidian incluído, edita o vault.
- O `tb agent` (`spawn --parent`, `list`, `approve`, `reject`, `budget`) também sai. As partes dele existem com outros nomes: admitir um agente é `tb bus invite` + `tb bus join`; rodar um é `tb bus run` ou `tb router run`; saúde é `tb watchdog` e `tb bus watchdog`. Approve/reject supunha uma fila de ações pendentes que não existe. O portão é o sandbox fail-closed e o mandato, não um prompt.
- O README, o `docs/arquitetura.md`, o `docs/cli.md` e o `docs/identidade-visual.md` deixam de apresentar os dois comandos como planejados. O histórico fica no ADR-006, no ADR-016 e neste ADR.

### 4. Roadmap

A linha do README "Hierarquia de agentes / A2A / `tb md` (posterior)" é trocada por um item aberto, a Fase A deste ADR. A2A e `tb md` ficam registrados como decididos.

## Consequências

### Positivas

- O roadmap diz o que falta, e os docs param de descrever um socket, um transporte HTTP e dois comandos que não serão construídos.
- O invariante de não usar rede continua absoluto para o `tb`. Todo salto de rede está num programa que o humano escolheu.
- A subdelegação fica visível e limitada sem processo, papel ou daemon novo: ela reusa claims, leases, épocas e revogações.

### Negativas

- Sem interoperabilidade A2A pronta. Quem quiser escreve uma ponte, e o contrato que ela segue é o protocolo do barramento, não o A2A.
- Um bump de schema: todo `tb` que serve um canal precisa ser atualizado junto, como no B2 e no B4.
- Os limites de profundidade e fan-out são grosseiros. Eles limitam delegação descontrolada; não limitam custo.
- Um pai pode ser completado com um filho ainda rodando. A Fase A só informa.

## Alternativas consideradas

- **Ponte A2A embutida (adapter `a2a` do `tb bus run`, ou `tb serve --a2a`).** Rejeitada: o `tb` teria um listener, e tarefas vindas da rede contornariam o consentimento por agente. O caminho do agente externo dá a mesma interoperabilidade com a rede fora do `tb`.
- **A2A como formato de fio do barramento.** Rejeitada: o A2A supõe endpoints HTTP e uma spec em evolução; agentes de fornecedores não expõem servidores (ADR-013). O envelope fica no formato A2A e versionado por `bus/0.1`.
- **Declarar a hierarquia pronta (router + papéis do barramento) e não acrescentar nada.** A mais simples. Rejeitada por ora porque a subdelegação já acontece por supervisores e, sem um vínculo de pai, ninguém a vê nem a limita. Continua sendo a saída se a Fase A não for desejada.
- **Herança de orçamento de tokens ou de tempo.** Adiada: o `tb` não vê o uso de tokens do fornecedor nem o tempo real de trabalho, só contagem de despertares, `ttl` e timestamps de claim. Um orçamento que ele não aplica seria documentação, não controle.
- **Papel de planner e `tb agent spawn`.** Rejeitada: planejar é trabalho do humano ou de um supervisor admitido, e criar agentes é `tb bus invite` + `tb bus run`. Um comando de spawn deixaria um agente criar outros, o que o modelo de convites proíbe.
- **Usar `causation_id` como pai.** Rejeitada: `causation_id` aponta para uma mensagem, não é checado e já tem um significado. Um vínculo de pai que só vale se for checado precisa ser um campo separado, validado pelo broker.

## Referências

- [Spec: árvores de delegação no barramento](../specs/agent-hierarchy.md)
- [ADR-002](002-mcp-a2a-protocolos.md), [ADR-009](009-router-e-workers.md), [ADR-013](013-barramento-multi-vendor.md), [ADR-016](016-vault-fase-a.md)
- [Protocolo do barramento](../bus-protocolo.md), [lease do barramento](../bus-lease.md), [harness do barramento](../bus-harness.md), [router no barramento](../bus-router.md)
- [Google A2A Protocol](https://github.com/google/A2A)
