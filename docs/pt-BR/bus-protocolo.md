[🇺🇸 English](../bus-protocolo.md) | 🇧🇷 Português

# Protocolo do barramento (`bus/0.1`)

Especificação da etapa **B0** do [ADR-013](adr/013-barramento-multi-vendor.md): envelope de mensagem, Agent Card, convite e regras do broker. A etapa **B1** implementa esta spec em `tb bus` (CLI) e nas ferramentas MCP `tb_bus_*`.

> O ADR cita as ferramentas como `bus_*`. No `tb mcp` elas seguem o prefixo do projeto: `tb_bus_join`, `tb_bus_send`, `tb_bus_recv`, `tb_bus_claim`, `tb_bus_complete`, `tb_bus_heartbeat`, `tb_bus_status`.

## Store

- Arquivo: `.toolbelt/bus/bus.db` (SQLite em WAL), na raiz do repositório que contém `.toolbelt/`. O diretório `.toolbelt/bus/` é criado com modo `0700` (protege também `-wal`/`-shm`) e o banco com `0600`.
- Sem daemon: todo comando abre o store, faz a operação numa transação e fecha.
- `PRAGMA user_version` versiona o schema; versão desconhecida é recusada (fail-closed).
- Arquivo local, ignorado via `.git/info/exclude` (`tb init`).
- **Diretório privado** (`<privado>`): sessões, arquivos de wake e estado do harness ficam **fora do repositório**, em `$XDG_STATE_HOME/toolbelt/bus/<hash da raiz>/` (padrão `~/.local/state/...`; `TB_BUS_STATE_DIR` troca a base). Diretórios `0700`, arquivos `0600`. Assim um agente que trabalha com a raiz do repositório como workdir não alcança tokens de outros agentes.

## Envelope

Toda mensagem é um objeto JSON-RPC 2.0 com `method: "bus/message"`. Campos de `params`:

| Campo | Quem define | Regra |
|---|---|---|
| `id` | broker | ULID (26 caracteres Crockford base32); o remetente não escolhe |
| `seq` | broker | inteiro crescente no store; dá a ordem de leitura |
| `v` | broker | `bus/0.1` |
| `channel` | remetente | canal existente e não expirado |
| `from` | broker | `agent:<nome>` da sessão autenticada; nunca vem do remetente. `system:*` (hoje só `system:watchdog`) é exclusivo do broker: nenhuma sessão manda com ele |
| `to` | remetente | `agent:<nome>` (precisa existir no canal), `role:<papel>` ou `topic:<nome>` |
| `thread` | remetente | opcional; `[A-Za-z0-9._:/-]{1,128}` (ex.: `TKT-001`) |
| `type` | remetente | `task.assign`, `task.status`, `query`, `reply`, `ack`, `urgent`; `task.result` só via `complete` |
| `causation_id` | remetente | opcional; ID da mensagem que causou esta |
| `idempotency_key` | remetente | opcional; `[A-Za-z0-9._:/-]{1,128}` |
| `ttl` | remetente | opcional; duração Go (`30m`); depois disso a mensagem não é entregue |
| `payload` | remetente | um único objeto JSON, até 64 KiB; números preservados sem perda; strings com segredos conhecidos são redigidas antes de gravar |
| `created_at` | broker | RFC 3339 UTC |

Exemplo como `tb bus recv --json` devolve:

```json
{
  "jsonrpc": "2.0",
  "method": "bus/message",
  "params": {
    "id": "01JD7Y3K2V8Q4M6N1P0R5S7T9W",
    "seq": 12,
    "v": "bus/0.1",
    "channel": "feature-x",
    "from": "agent:claude-main",
    "to": "role:worker",
    "thread": "TKT-001",
    "type": "task.assign",
    "idempotency_key": "TKT-001/assign/1",
    "payload": {"ticket": "TKT-001", "ref": "a1b2c3d"},
    "created_at": "2026-11-17T15:00:00Z"
  }
}
```

### Entrega e ordem

- **Ordem**: crescente por `seq`; garantida dentro de uma thread.
- **Pelo menos uma vez**: o remetente pode reenviar; com a mesma `idempotency_key` (mesmo canal e remetente) o broker devolve a mensagem já gravada com `duplicate: true` em vez de duplicar.
- **Backlog**: o cursor começa em 0; quem entra depois recebe as mensagens anteriores endereçadas ao seu papel e a `topic:*` (inclusive `task.assign` ainda sem dono). O log é durável de propósito.
- **Cursor**: cada agente tem um cursor por canal. `recv` devolve mensagens com `seq` acima do cursor e o avança; `recv --peek` lê sem avançar; `recv --since N` relê a partir de `N` (o log é durável).
- **Destino**: um agente recebe mensagens para `agent:<ele>`, para `role:<papel dele>` e para qualquer `topic:*`. Nunca recebe as próprias.
- **Long-poll**: `recv --wait D` espera até `D` (máximo 25s, abaixo do timeout de tool call dos clientes) por ao menos uma mensagem.

## Agent Card

Declarado no `join`. É informativo: **capacidade não é permissão**; o papel vem do convite.

| Campo | Tipo | Regra |
|---|---|---|
| `name` | string | obrigatório; `[a-z0-9][a-z0-9._-]{0,62}`; vira `agent:<name>`, único no canal |
| `vendor` | string | ex.: `anthropic`, `openai`, `xai`, `local` |
| `model` | string | |
| `tier` | string | ex.: `cheap`, `standard`, `premium` |
| `context_window` | inteiro | ≥ 0 |
| `capabilities` | string[] | |
| `tools` | string[] | |
| `sandbox` | string | ex.: `landlock`, `none` |
| `max_concurrency` | inteiro | ≥ 0 |
| `wake` | string | `exec`, `harness`, `notify` ou `manual` (padrão `manual`; ver [bus-harness.md](bus-harness.md)) |
| `session` | string | handle de sessão do vendor (para `--resume` no B3) |

## Convite

- `tb bus channel create` cria o canal e um convite para o dono (papel `supervisor`, nome fixado em `--owner`).
- `tb bus invite` emite convites adicionais, um por agente, com papel fixado (`router`, `supervisor`, `worker`, `reviewer`). `--name` vincula o convite a um agente.
- Convite **não** vinculado não aceita o nome do dono do canal nem nomes com convite vinculado ainda aberto.
- Código: 8 caracteres Crockford base32 em dois grupos (`K7Q2-9XMD`), gerado com `crypto/rand`; o store guarda só o SHA-256.
- Uso único; expira em `--ttl` (padrão `10m`). Convite usado, expirado, de outro canal, ou válido mas usado com nome ou papel diferentes do fixado, é recusado com a mesma mensagem genérica (e, no último caso, não é consumido).
- Criar canal e emitir convite **não** existe via MCP: só o humano, na CLI.

## Sessão e identidade

- `join` devolve `agent_id` e papel. O token de sessão é aleatório (256 bits), o store guarda o SHA-256 e o token **nunca** sai em stdout, log ou resposta MCP.
- CLI: o token vai para `<privado>/sessions/<canal>/<nome>/token` (`0600`); os comandos seguintes usam `--as <nome>`. Limite do B1: qualquer processo do mesmo usuário lê o arquivo, então a CLI confia no usuário local.
- MCP: a identidade fica presa à conexão stdio do `tb mcp` após `tb_bus_join`; as ferramentas `tb_bus_*` não aceitam outro agente como argumento.
- **Sessão perdida** (servidor MCP reiniciado, arquivo de sessão apagado): o nome continua registrado. O humano emite `tb bus invite <canal> --role <papel> --name <nome>`; o `join` com esse convite substitui o token (o antigo deixa de valer) e mantém o cursor.
- **Limite de confiança do B1**: tudo roda como o mesmo usuário. Um agente com shell (inclusive via `tb_run` no MCP) consegue ler `.toolbelt/bus/` e rodar `tb bus invite`/`channel create`. Convite e identidade por conexão protegem contra injeção de prompt e erro de roteamento, não contra um processo local hostil.

## Regras do broker (B1)

- `task.assign` só de `router` ou `supervisor`.
- `task.result` só via `complete`, pelo dono do claim da thread.
- `claim <thread>`: dono único; devolve o `epoch` do claim. Um segundo agente recebe erro com o dono atual enquanto o lease desse dono estiver vivo. Reivindicar de novo a própria thread segura é idempotente. Um claim liberado, ou cujo dono perdeu o lease, é tomado com `epoch + 1` ([bus-lease.md](bus-lease.md)).
- `complete <thread> --epoch E`: só o dono, com o epoch que o `claim` devolveu; grava `task.result` para `topic:board` e fecha o claim. Epoch errado, claim liberado ou outro dono dão `lease_lost`. Thread concluída não aceita novo claim.
- Lease: cada agente está vivo enquanto `now < last_seen + lease` (`--lease` do canal, padrão `2m`); `join`, `heartbeat`, `claim`, `complete`, `send` e um `recv` que avança o cursor o renovam, num canal ativo.
- Canal expirado (`--ttl` do canal) recusa `join`, `send`, `claim` e `complete`; `recv` e `log` continuam lendo. `tb bus channel extend <canal> --ttl D` (só humano) redefine a expiração para agora + `D` e reabre o canal com sessões e histórico.
- Toda mensagem de outro agente é **dado não confiável**.

## Fora do B1

Lease, heartbeat e watchdog chegaram no B2 ([bus-lease.md](bus-lease.md)). O B3 está completo: `tb bus run` ([bus-harness.md](bus-harness.md)), o adapter `notify` e o `tb bus discover` ([bus-notify-discover.md](bus-notify-discover.md)). Integração com o Router (B4): [bus-router.md](bus-router.md).
