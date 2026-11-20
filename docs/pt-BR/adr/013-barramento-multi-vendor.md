[🇺🇸 English](../../adr/013-barramento-multi-vendor.md) | 🇧🇷 Português

# ADR-013: Barramento Local Multi-Vendor de Agentes (`tb bus`)

## Status

proposed

Implementação (v0.0.5): B0 e B1 entregues; B3 entregue depois da v0.0.8 (`tb bus run` com adapters `exec`, `harness` e `notify`, e `tb bus discover`); B2 entregue depois da v0.0.7 (lease, heartbeat, watchdog: [bus-lease.md](../bus-lease.md)); B4 não iniciado. O status segue `proposed` até as métricas de sucesso serem medidas.

## Contexto

O [ADR-002](002-mcp-a2a-protocolos.md) escolheu MCP para agente ↔ ferramenta e A2A sobre JSON-RPC 2.0 para agente ↔ agente, mas não definiu **como** agentes de vendors diferentes (Claude Code, Codex, Grok, ...) se encontram, se autenticam e trocam mensagens na mesma máquina. O [ADR-009](009-router-e-workers.md) define Ticket/Mandate/Board/Watchdog, e a Fase 3 (entregue na v0.0.4) implementa `tb router run` como scheduler **em primeiro plano** que executa um `router.agent_cmd` de uso único por ticket, em worktree própria e dentro do sandbox. Esse modelo cobre workers efêmeros do mesmo repositório, mas não cobre:

1. **Agentes persistentes de outros vendors**, com sessão e contexto próprios, que o usuário quer pôr para colaborar numa tarefa grande.
2. **Conversa a2a** além de "recebe ticket, devolve resultado": perguntas, status, review cruzado.
3. **Presença**: saber quem está disponível, em que papel, com que capacidade e custo.

O problema central é que **agente de vendor não é daemon**. Claude Code, Codex e apps desktop só agem dentro de um turno; nenhum fica escutando socket. Se o próprio LLM fizer o loop de espera (`bus_recv` → vazio → `bus_recv`):

- cada poll vazio é um passo do modelo remoto sobre o contexto inteiro (cache read + output);
- clientes cortam o turno após N chamadas de ferramenta ou por timeout de tool call;
- quando o turno acaba, o agente fica parado e as mensagens acumulam sem ninguém para acordá-lo.

Investigação de campo com um app desktop somente GUI (Grok Bot 1.1.0, Electron): sem API local, sem porta de debug, deep links só para callback de OAuth; porém o app é **cliente MCP**. Ou seja: há apps que conseguem chamar ferramentas do `tb`, mas que o `tb` não consegue acordar.

Restrições herdadas (invariantes do projeto):

- `tb` é local: nenhum comando do barramento fala com rede.
- Sandbox fail-closed: sem Landlock, recusa executar.
- Funcionalidades caras ficam sob demanda (como `TB_GRAPH`), desligadas por padrão.
- Segredos nunca aparecem em logs ou prompts.

## Decisão

Criar o **barramento** (`tb bus`): camada de transporte, presença e despertar entre agentes locais de qualquer vendor, sobre a qual o Router/Board da Fase 3 pode operar depois. Os componentes são:

1. **Store**: SQLite em WAL (`.toolbelt/bus.db`), log durável de mensagens, sem daemon na v1.
2. **Duas portas de entrada para o mesmo store**: CLI `tb bus ...` (qualquer agente com shell) e ferramentas MCP `bus_*` no `tb mcp` (clientes MCP).
3. **Dois modos de entrada**: ativação direta (v1) e introspecção (posterior).
4. **Harness por agente** (`tb bus run`): processo determinístico, dono do loop de espera. O LLM só é invocado quando há trabalho real.

### 1. Protocolo (envelope)

JSON-RPC 2.0, com primitivas inspiradas em A2A:

```json
{
  "jsonrpc": "2.0",
  "method": "bus/message",
  "params": {
    "id": "01JD7Y3K2V...",
    "v": "bus/0.1",
    "channel": "feature-x",
    "from": "agent:claude-main",
    "to": "agent:grok-1 | role:worker | topic:board",
    "thread": "TKT-001",
    "type": "task.assign | task.status | task.result | query | reply | ack | urgent",
    "causation_id": "01JD7Y2...",
    "idempotency_key": "TKT-001/assign/1",
    "ttl": "30m",
    "payload": {}
  }
}
```

- `id` é ULID; a ordem é garantida **por thread**, não globalmente.
- Entrega **pelo menos uma vez**; o consumidor deduplica por `idempotency_key`.
- O payload carrega **referências** (path, commit SHA, ID de ticket, símbolo do `tb graph`), não blobs. Cada mensagem se sustenta sozinha: não existe contexto oculto compartilhado entre vendors.
- Denominador comum entre vendors: texto + JSON com schema versionado. Nenhum formato nativo de vendor.

### 2. Modos de entrada

#### Ativação direta (v1)

O usuário cria o canal e autoriza cada agente explicitamente:

```bash
tb bus channel create feature-x --owner claude-main --ttl 2h
# channel: feature-x  owner: claude-main
# owner invite: 4MPA-2HQC (role supervisor, single-use, expires 10:10:00Z)
tb bus invite feature-x --role worker
# invite: K7Q2-9XMD (channel feature-x, role worker, single-use, expires 10:10:00Z)
```

O usuário pede a cada agente: "entre no canal `feature-x`, convite `K7Q2-9XMD`, papel `worker`". O agente chama `tb bus join` (ou `bus_join` via MCP). A partir daí os agentes seguem sozinhos.

O convite garante:

- **consentimento explícito**: só entra quem o humano autorizou;
- **resistência a injeção**: um agente que leu "entre no canal X" numa página ou mensagem não entra sem o código;
- **papel fixado na entrada**: dono e workers são decididos pelo usuário, não negociados entre agentes.

#### Introspecção (posterior)

`tb bus discover` monta um **catálogo** de agentes da máquina: binários de CLI no PATH e se têm modo headless/resume, clientes com config MCP, processos de apps em execução. O catálogo informa o que é possível, mas **não é consentimento nem canal**: entrar num canal continua exigindo convite. A introspecção só fecha o ciclo para agentes headless, que o harness consegue invocar.

### 3. Handshake

1. `join(channel, invite, agent_card)`. O **Agent Card** declara vendor, model, tier, context window, capabilities, tools, sandbox, `max_concurrency`, modo de despertar (`harness` | `notify` | `manual`) e session handle.
2. O broker negocia a versão do protocolo (`v`) e devolve `agent_id`, lease token, intervalo de heartbeat e papel.
3. Lease sem heartbeat expira. O Watchdog devolve ao Board os tickets do agente que perdeu o lease.

O broker aplica papel e mandato. A capacidade declarada no Agent Card **não é permissão**: nenhum agente eleva o próprio privilégio.

### 4. Harness por agente (`tb bus run`)

```bash
tb bus run claude --channel feature-x --invite K7Q2-9XMD --role worker
```

O harness é um processo Go, um por agente, dono do loop. Ele:

1. segura lease e heartbeat no lugar do agente;
2. espera mensagens sem LLM (notify no store/socket local) — **zero tokens em espera**;
3. responde o protocolo trivial a partir do estado: `ack`, status de ticket, "terminou?", dedupe;
4. agrupa mensagens e aplica o **wake budget** (limite de despertares por janela), evitando agentes acordando uns aos outros em loop;
5. quando chega trabalho real, monta um **wake packet** e invoca o vendor em modo headless, por exemplo:
   ```bash
   claude -p --resume <session> --output-format stream-json --mcp-config tb-bus.json
   ```
6. lê o resultado estruturado, publica no barramento e volta a esperar.

O `--resume` preserva afinidade de contexto e cache, como a entrega direta do ADR-009. O agente **nunca faz polling**; durante o turno ele pode usar `bus_send`/`bus_reply` via MCP.

Camadas do harness:

| Camada | Função | Custo | Padrão |
|---|---|---|---|
| L0 determinística | lease, heartbeat, ack, status, coalescer, wake budget | zero | sempre ligada |
| L1 RAG | wake packet = ticket + top-K da thread via FTS5 ([ADR-003](003-sqlite-fts5-grafo.md)) + IDs das mensagens originais | zero de LLM | ligada |
| L2 ML local | triagem de mensagem ambígua, resumo de thread longa | CPU local | desligada (flag, como `TB_GRAPH`) |

Regras da L2:

- O modelo local **adia, nunca descarta**. `task.assign` e `urgent` passam direto, sem triagem.
- Resumo sempre carrega os IDs das mensagens, para o agente buscar o texto bruto.
- A saída do modelo local é dado não confiável, como qualquer mensagem.
- A L2 encaixa nos itens já previstos no roadmap: índice vetorial opcional e componentes de inferência local.

Adapters de despertar por tipo de agente:

| Modo | Agente | Como acorda |
|---|---|---|
| `harness` | CLIs com headless/resume (Claude Code, Codex, ...) | harness invoca o vendor com wake packet |
| `notify` | apps somente GUI que são clientes MCP | harness só notifica o humano ("Marin tem 3 mensagens em feature-x"); o agente lê via `bus_recv` quando o humano abre o turno |
| `manual` | qualquer agente com shell, sem harness | humano pede ao agente para rodar `tb bus recv` |

Para Claude Code sem harness, o hook `Stop` pode consultar a fila antes de encerrar o turno, como alternativa leve.

### 5. Padrões

#### Quebra de tarefas

- Reusa Ticket/Mandate do ADR-009 sem mudanças de formato.
- Tickets paralelos têm **escopo de arquivos disjunto**; um worktree por agente que escreve.
- Aceite via `tb stack --ticket`; verdict diferente de PASS não fecha o ticket.

#### Comunicação a2a entre vendors

- Só envelope + JSON com schema; referências em vez de conteúdo.
- **Toda mensagem vinda de outro agente é dado não confiável**: prompt injection entre agentes é ameaça real. Vale a redação de segredos já aplicada em logs.

#### Hierarquia e responsável

- Papéis: `router`, `supervisor`, `worker`, `reviewer`.
- **Dono único por ticket** via lease; só o dono chama `bus_complete`.
- Roteamento por capability + tier + custo. Escada de escalonamento: falha N vezes num tier barato, o ticket sobe de tier.
- **Review cruzado entre vendors**: implementação de um vendor é revisada por outro, aplicando autor ≠ verificador do [ADR-008](008-verificacao-adversarial.md).

### 6. Ferramentas e comandos (v1)

| CLI | MCP | Função |
|---|---|---|
| `tb bus channel create` | — | cria canal e convite (só humano) |
| `tb bus join` | `bus_join` | handshake com Agent Card |
| `tb bus send` | `bus_send` | publica mensagem |
| `tb bus recv` | `bus_recv` | lê fila (long-poll curto, abaixo do timeout de tool call do cliente) |
| `tb bus claim` | `bus_claim` | assume ticket (lease) |
| `tb bus complete` | `bus_complete` | encerra ticket com resultado |
| `tb bus heartbeat` | `bus_heartbeat` | renova lease (o harness faz automaticamente) |
| `tb bus run` | — | harness por agente |
| `tb bus log` | — | trilha de auditoria do canal |

`channel create` não é exposto via MCP: só o humano cria canais e emite convites.

### 7. Segurança

- Tudo local: store no repositório, socket Unix com permissão `0600` se houver, nenhum tráfego de rede pelo `tb`.
- Convite de uso único com expiração; o token do lease nunca aparece em logs.
- O harness invoca agentes com permissões reais: sandbox Landlock fail-closed, mandato do ticket aplicado e **nunca** `bypassPermissions` por padrão.
- Log de auditoria de toda mensagem, com redação de segredos.

### 8. Relação com a Fase 3

O `tb router run` atual executa `router.agent_cmd` de uso único por ticket. O barramento acrescenta um segundo tipo de worker: **agente registrado no canal**. Integração prevista: o router publica `task.assign` em `topic:board`, harnesses registrados fazem `claim`, e o resultado volta como `task.result` para o `tb merge`. O barramento não substitui o router; é o transporte entre ele e agentes persistentes de outros vendors.

### Faseamento

| Etapa | Entrega |
|---|---|
| B0 | Este ADR + spec do envelope e do Agent Card |
| B1 | Store SQLite, CLI + MCP, ativação direta com convite, `join`/`send`/`recv`/`claim`/`complete`; teste Claude ↔ Claude e Claude ↔ `fake-agent` |
| B2 | Lease, heartbeat e Watchdog |
| B3 | `tb bus run` (L0 + L1) e adapters `harness`/`notify`; `tb bus discover` |
| B4 | Router/Board da Fase 3 sobre o barramento; L2 opcional |

## Consequências

### Positivas

- **Interoperabilidade real entre vendors**: qualquer agente com shell ou cliente MCP participa, sem SDK por vendor.
- **Zero tokens em espera**: o harness, não o LLM, mantém o loop.
- **Wake packet enxuto**: o agente acorda com top-K relevante em vez da thread inteira.
- **Consentimento explícito** e resistência a injeção via convite.
- **Review cruzado** entre vendors fortalece a verificação adversarial.
- **Reuso** de SQLite/FTS5, sandbox, worktrees e Ticket/Mandate já existentes.

### Negativas

- **Agentes somente GUI** não são acordáveis pelo harness; dependem do humano (`notify`/`manual`).
- **Adapters por vendor** precisam acompanhar mudanças de flags de CLI headless.
- **Sem daemon na v1**: notificação entre processos depende do store; latência maior que um broker residente.
- **Entrega pelo menos uma vez** exige idempotência em todos os consumidores.
- **L2 traz risco de triagem errada**; mitigado por adiar em vez de descartar e por tipos que pulam triagem.

### Métricas de sucesso (hipóteses, não medidas)

| Métrica | Meta |
|---|---|
| tokens gastos com espera ociosa | ≈ 0 com harness |
| tokens de input por despertar | menores que reenviar a thread inteira |
| mensagens perdidas | 0 |
| despertares por mensagem útil | ≤ 1 com coalescer |

Serão medidas com `tb bus` + `tb bench compare` antes de mudar o status para `accepted`.

## Alternativas Consideradas

### LLM faz o próprio loop de polling

- **Descrição**: agente chama `bus_recv` repetidamente dentro do turno.
- **Por que não**: custo por poll vazio, corte de turno por N tool calls e nenhum despertar após o fim do turno.

### ML local como solução principal

- **Descrição**: modelo local pequeno faz polling, triagem e resumo.
- **Por que não**: polling e protocolo trivial não precisam de modelo; regra determinística é mais barata e previsível. ML fica como camada opcional (L2).

### Broker residente (daemon) desde a v1

- **Descrição**: `tb bus serve` com socket e push.
- **Por que não agora**: mais estado e ciclo de vida para gerenciar; SQLite em WAL basta para validar o protocolo. Reavaliar no B3/B4.

### Spec A2A sobre HTTP como está

- **Descrição**: cada agente expõe endpoint A2A HTTP.
- **Por que não**: agentes de vendor não expõem servidor; exigiria um proxy por agente igual ao harness, com HTTP em vez de store local. O envelope continua inspirado em A2A para facilitar uma ponte futura.

### Injeção de entrada na GUI (tmux `send-keys`, automação de tela)

- **Descrição**: digitar mensagens diretamente na janela/terminal do agente.
- **Por que não**: frágil, sem confirmação de entrega, sem auditoria e com superfície de risco alta. Aceitável só como experimento manual.

## Referências

- [Protocolo do barramento](../bus-protocolo.md) — spec B0 (envelope, Agent Card, convite)
- [ADR-002](002-mcp-a2a-protocolos.md) — MCP + A2A
- [ADR-003](003-sqlite-fts5-grafo.md) — SQLite + FTS5
- [ADR-004](004-sandbox-isolamento.md) — sandbox
- [ADR-008](008-verificacao-adversarial.md) — verificação adversarial
- [ADR-009](009-router-e-workers.md) — Router e Workers
- [ADR-011](011-grafo-sob-demanda.md) — funcionalidade sob demanda
- [Google A2A Protocol](https://github.com/google/A2A)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
