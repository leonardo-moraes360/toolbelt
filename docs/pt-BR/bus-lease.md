[🇺🇸 English](../bus-lease.md) | 🇧🇷 Português

# Lease, heartbeat e watchdog do barramento — spec B2

Status: **implementado** (B2).

Especificação da etapa **B2** do [ADR-013](adr/013-barramento-multi-vendor.md). Estende o protocolo [`bus/0.1`](bus-protocolo.md) (B1) e o harness ([bus-harness.md](bus-harness.md), primeiro corte do B3).

## Problema

No B1 um claim nunca expira. Se o dono de um thread trava, perde a sessão ou só é fechado, o thread fica com claim para sempre: nenhum outro agente consegue pegá-lo e nada avisa o humano. O ADR-013 pede três peças:

- um **lease** sobre o que um agente segura;
- um **heartbeat** que o renova;
- um **watchdog** que devolve ao board o trabalho de um agente que perdeu o lease.

## Princípios

1. **Sem daemon.** O store não tem processo de fundo. A expiração é um fato calculado a partir de timestamps quando alguém olha: um claim, um complete, um heartbeat ou uma rodada do watchdog.
2. **A vivacidade é por agente, não por claim.** Um heartbeat mantém todos os claims daquele agente no canal.
3. **Fencing por epoch.** Cada claim tem um `epoch` que sobe toda vez que o thread recebe um claim depois de uma liberação ou de uma tomada. O `complete` precisa apresentar o epoch que recebeu do `claim`, então um dono desatualizado, mesmo com o mesmo nome, nunca conclui um trabalho que trocou de mãos.
4. **Nada se perde.** Um claim expirado é liberado, não apagado, e o thread pode receber claim de novo; o log guarda o histórico.
5. **Zero tokens parado continua valendo.** Nenhum LLM precisa mandar heartbeat: o harness e o servidor MCP mandam pelas sessões que seguram.
6. **Mesma fronteira de confiança do B1.** Um heartbeat é autenticado como qualquer chamada: o token de sessão é o token do lease. Um agente não consegue manter vivo o lease de outro, e um processo hostil do mesmo usuário continua fora do escopo.

## Modelo

- **Canal:** uma duração `lease` definida por `tb bus channel create --lease D` (padrão `2m`, mínimo `30s`, máximo `1h`). O B2 não tem como mudá-la depois da criação. O intervalo de heartbeat é `lease / 3`.
- **Agente:** `last_seen`, limitado a agora na leitura (um relógio que volta não estende um lease). O agente está **vivo** enquanto `now < last_seen + lease`.
- **Claim:**
  - ganha `epoch` (a partir de 1), `released_at` e `prev_owner` (o dono antes da última tomada);
  - está **seguro** quando não está concluído nem liberado e o dono está vivo;
  - está **perdido** para um agente quando `prev_owner` é esse agente, ou quando o agente ainda é o dono e `released_at` está preenchido.

### O que renova `last_seen`

| Operação | Renova |
|---|---|
| `join` (e uma troca de sessão), `heartbeat`, `claim`, `complete`, `send` | sim |
| `recv` que avança o cursor, num canal ativo | sim |
| `recv --peek`, um `recv --wait` vazio, `log`, status, qualquer chamada num canal expirado | não |

## Regras

- **R1, join:** o `join` devolve `lease_ends_at` e `heartbeat_interval` junto com a sessão (ADR-013 §3).
- **R2, claim:** o resultado traz o `epoch`.
  - Um thread que ninguém segura recebe claim com `epoch` 1.
  - Um thread cujo dono está vivo mantém o erro do B1, com o nome do dono e a hora em que o lease dele acaba. Repetir o claim de um thread seguro pelo próprio agente é idempotente.
  - Um thread cujo claim foi liberado, ou cujo dono não está vivo, é tomado por quem chama (inclusive o mesmo agente, para um claim liberado; um dono que chama `claim` volta a estar vivo e mantém o epoch). O claim ganha `epoch + 1`, `prev_owner` = o dono anterior (quando era outro agente) e `released_at` limpo. Se o claim ainda não tinha sido liberado, a tomada publica o evento de liberação do R5.
  - Duas tomadas concorrentes rodam em transações `IMMEDIATE`: exatamente uma vence, a outra recebe o erro de "já tem claim".
- **R3, complete:** exige o epoch (`tb bus complete --epoch E`, `epoch` no `tb_bus_complete`). É recusado com `lease_lost`, sem gravar nada, quando:
  - quem chama não é o dono atual;
  - o epoch é diferente do do claim;
  - ou o claim está liberado, mesmo que pelo watchdog e sem ninguém tê-lo tomado. O board já viu o `lease_expired`, então o agente precisa fazer claim de novo.

  Um dono cujo lease acabou, mas cujo claim ninguém liberou nem tomou, ainda pode concluir; a chamada renova o `last_seen` dele.
- **R4, heartbeat:** `tb bus heartbeat --channel C --as N` e a tool MCP `tb_bus_heartbeat`. Cada um define `last_seen` e devolve:
  - `lease_ends_at` e `heartbeat_interval`;
  - os threads que o agente segura, com os epochs;
  - os threads que ele **perdeu**, com o novo dono quando houver.

  Um heartbeat nunca ressuscita um claim liberado.
- **R5, watchdog:** `tb bus watchdog C` (só CLI, humano ou router) libera todo claim que não está concluído nem liberado e cujo dono não está vivo. Para cada um publica um `task.status` em `topic:board`:

  ```json
  {"state":"lease_expired","thread":"T","previous_owner":"agent:N","epoch":E,"last_seen":"RFC3339"}
  ```

  - O remetente é `system:watchdog`. `system:*` é um remetente exclusivo do broker que nenhuma sessão pode usar, e a regra do `from` do bus-protocolo.md é ajustada para permiti-lo. Os consumidores só confiam num `lease_expired` vindo de `system:watchdog`; o mesmo texto vindo de um agente é só uma mensagem não confiável.
  - É idempotente: um claim já liberado não publica nada.
  - `--dry-run` lista sem liberar; `--json` imprime a lista.
  - Não fica exposto no MCP, para que um agente não consiga liberar o trabalho de outros. O R2 libera sob demanda, então o watchdog só importa quando ninguém faz claim do thread.
- **R6, harness:** o `tb bus run` manda heartbeat a cada intervalo numa goroutine própria, independente do loop de observação e de um wake, então um wake bloqueado pelo timeout inteiro mantém o lease.
  - Registra `lease_lost` (thread e novo dono, nunca um payload) quando um heartbeat informa uma perda nova.
  - Com `--once`, nada manda heartbeat depois que a execução termina: os claims então duram um lease.
  - O harness nunca faz claim nem complete por conta própria.
- **R7, servidor MCP:** o `tb mcp` manda heartbeat, num timer de fundo, para toda sessão do barramento ligada à conexão stdio enquanto ela estiver aberta. Agentes com `wake: manual` ou `notify` mantêm o lease sem gastar tokens; fechar o cliente encerra os heartbeats.
- **R8, status:**
  - `tb bus channel show C` passa a mostrar os agentes (nome, papel, vivo, `last_seen`) e os claims abertos (thread, dono, epoch, `state` seguro, expirado, liberado ou revogado, e `outcome`: nulo enquanto seguro, `expired` quando o lease do dono venceu ou o watchdog liberou, `revoked` quando um supervisor encerrou; claims completados saem da lista de abertos).
  - A tool MCP `tb_bus_status` devolve o mesmo para o canal de quem chama.

  Nenhum dos dois devolve Agent Cards (o campo `session` é um handle de retomada do vendor), tokens nem payloads.
- **R9, canal expirado:** um canal depois do `--ttl` recusa `heartbeat`, `claim`, `complete` e `watchdog` como qualquer escrita, e nada renova `last_seen` ali. O `channel extend` o reabre com os timestamps antigos, então agentes que não mandaram heartbeat nesse meio-tempo voltam como não vivos.

## Schema e upgrade

- O `PRAGMA user_version` vai de 1 para 2. O `migrate` ganha um passo `1 → 2` na transação `IMMEDIATE` que já existe:
  - `channels.lease_ns INTEGER NOT NULL DEFAULT 120000000000`;
  - `agents.last_seen INTEGER`, preenchido com a **hora da migração** para os agentes existentes, para que um upgrade não faça todo mundo expirar de uma vez;
  - `claims.epoch INTEGER NOT NULL DEFAULT 1`, `claims.released_at INTEGER`, `claims.prev_owner TEXT`.
- Um store novo já nasce na v2.
- A linha `(channel, thread)` continua sendo o claim atual. Uma tomada sobrescreve `owner`, `claimed_at` e `epoch`, define `prev_owner` e limpa `released_at`; o histórico fica no log.
- O upgrade é **de mão única**. Qualquer `tb` que abre o store (até `tb bus log`) o migra, e um binário mais antigo passa a recusá-lo, por exemplo um `tb mcp` de release rodando ao lado de um build local. Atualize juntos todos os `tb` que servem um canal.
- `complete` sem epoch passa a ser erro de uso. É uma mudança da API do B1 para a CLI e o `tb_bus_complete`; o prompt do harness diz ao agente para passar o epoch que recebeu do `claim`.

## Fora do B2

- **Board e redistribuição (B4):** o watchdog anuncia; não escolhe o próximo agente nem reenvia `task.assign`.
- **Mudar o lease de um canal existente, leases por claim, durações por agente, prioridades.**
- **Fontes de relógio:** hora de parede da máquina local, limitada como acima; todo processo roda num só host, como no B1.

## Critérios de aceite

1. Com o relógio adiantado além do lease, um segundo agente faz claim do thread de um dono calado e recebe `epoch` 2. O `topic:board` recebe um `lease_expired` de `system:watchdog`.
2. Dono A → B → A dá epoch 3. Um `complete` com epoch 1 falha com `lease_lost` e não grava nada. Antes de qualquer tomada, um `complete` atrasado com o epoch certo passa.
3. Depois de o `tb bus watchdog` liberar o claim de A sem ninguém tomá-lo, o `complete` de A falha com `lease_lost`. A faz claim de novo e recebe epoch 2.
4. Duas tomadas concorrentes do mesmo thread: exatamente uma vence.
5. Um heartbeat mantém vários claims seguros. `recv` só de peek ou vazio, e qualquer chamada num canal expirado, não renovam `last_seen`.
6. Um heartbeat informa os threads seguros, com epochs, e os perdidos, com os novos donos.
7. O `tb bus watchdog` libera só claims de agentes não vivos, publica uma vez por claim, e `--dry-run` não muda nada.
8. Nenhuma sessão consegue mandar como `system:watchdog` (nomes de agente não podem ter `:`, e `system:*` é exclusivo do broker).
9. O harness manda heartbeat a cada `lease / 3` com relógio falso, inclusive durante um wake bloqueado pelo timeout, e registra `lease_lost` quando um claim é tomado.
10. O `tb mcp` mantém viva uma sessão ligada sem chamadas de tool e para quando a conexão fecha.
11. Migrar um store v1 preserva canais, agentes (vivos, `last_seen` = hora da migração), cursores, mensagens e claims (`epoch` 1). Um store v3 é recusado.
12. `channel show`, `tb_bus_status`, a saída do `watchdog` e o log do harness não têm o `session` do Agent Card, token, código de convite nem payload.
13. `tb_bus_heartbeat` e `tb_bus_status` existem no MCP com paridade. Nenhuma tool MCP roda o watchdog.
