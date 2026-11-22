[🇺🇸 English](../../specs/agent-hierarchy.md) | 🇧🇷 Português

# Spec: árvores de delegação no barramento, Fase A (`task.assign --parent`, `tb bus tree`)

Autor: toolbelt · Status: Aceita, Fase A implementada (questões abertas resolvidas em 2026-11-22) · Data: 2026-11-22 · Decisão: [ADR-020](../adr/020-agent-hierarchy-a2a.md) · Base: [bus-protocolo.md](../bus-protocolo.md) (B1), [bus-lease.md](../bus-lease.md) (B2), [bus-router.md](../bus-router.md) (B4) · Relacionadas: [ADR-009](../adr/009-router-e-workers.md), [ADR-013](../adr/013-barramento-multi-vendor.md)

## Visão geral

Um supervisor num canal do barramento já pode dar claim numa thread e enviar `task.assign` para subtrabalho, mas o barramento não registra que a subthread pertence à primeira. Esta spec acrescenta um vínculo de pai checado pelo broker, limites de profundidade e fan-out e uma visão de árvore só leitura. Não acrescenta papel, processo, daemon nem rede. A Fase A é um PR.

## Estado atual (2026-11-22)

| Item | Estado |
|------|--------|
| `task.assign` | Só de `router` ou `supervisor` (`validateSend`, `internal/bus/store.go`) |
| `thread`, `causation_id` | Livres, validados só como `[A-Za-z0-9._:/-]{1,128}`; nenhum vínculo entre threads |
| Claims | Uma linha por `(channel, thread)`: dono, época, completada, liberada, revogada (schema v3) |
| `Revoke` | `router` ou `supervisor`, qualquer thread; API interna usada pelo router, sem CLI |
| Visões | `tb bus log` (mensagens), `tb bus channel show` (agentes, claims abertos); sem árvore |
| Schema do store | `PRAGMA user_version = 3` |

## Objetivos

- G1. Um `task.assign` pode nomear uma thread pai, e o broker só o aceita de quem tem direito de delegar esse pai.
- G2. A delegação é limitada: uma profundidade máxima e um número máximo de filhos abertos por pai.
- G3. O humano vê cada árvore de delegação e o estado de cada nó, sem payloads.
- G4. Completar um pai com filhos abertos é informado a quem completa.

## Não objetivos

- Herança de orçamento de tokens, custo ou tempo (ADR-020: o `tb` não consegue medir).
- Papel de planner, `tb agent spawn`, ou qualquer forma de um agente criar agentes, canais ou convites.
- Recusar `complete` num pai com filhos abertos (só informa, resolvido na Q2).
- Revogação em cascata de uma subárvore, ou uma CLI de revogação.
- Mudanças no router (B4): o router continua enviando assigns sem pai, e assigns com pai nunca são tarefas de router.
- Mudanças no wake packet ou no prompt do harness.
- Transporte A2A de qualquer tipo.

## Jornadas críticas do usuário

1. **Supervisor divide um ticket.** `claude-main` (supervisor) dá claim em `TKT-001` e envia dois `task.assign` com `--parent TKT-001` nas threads `TKT-001/api` e `TKT-001/ui` para `role:worker`. Workers dão claim e completam. `claude-main` completa `TKT-001`.
2. **Humano inspeciona.** `tb bus tree feature-x` mostra `TKT-001` com os dois filhos, os donos e os estados.
3. **Delegação descontrolada.** Um supervisor com mau comportamento tenta um nono filho aberto ou um quarto nível; o broker recusa com `fanout_exceeded` ou `depth_exceeded`.
4. **Pai sem direito.** Um supervisor que não tem `TKT-001` envia `--parent TKT-001`; o broker recusa com `parent_not_owned`.

## Requisitos

### Envelope

- **R1.** O `bus/0.1` ganha um campo opcional `parent`, definido pelo remetente, com a mesma sintaxe de `thread`. Só é aceito em `task.assign` (`parent_requires_assign` caso contrário) com `thread` não vazio (`parent_requires_thread`) e diferente de `parent` (`parent_self`). A versão do protocolo continua `bus/0.1`: o campo é aditivo e ausente de toda mensagem existente.
- **R2.** A saída de `recv`, `log` e `--json` traz `parent` nas mensagens que o têm.

### Checagens do broker (dentro da transação do `Send`)

Rodam depois da busca de idempotência (uma duplicata devolve a mensagem guardada, como hoje) e antes da inserção.

- **R3. Pai conhecido.** `parent` precisa ter pelo menos um `task.assign` anterior no canal (`parent_unknown`).
- **R4. Remetente com direito.** O remetente é o `router`, ou tem o claim vivo de `parent`: o `owner` da linha do claim é o remetente e `completed_at`, `released_at` e `revoked_at` são nulos (`parent_not_owned`). Um supervisor que perdeu o lease tem `released_at` definido depois do watchdog ou de um takeover, e é recusado.
- **R5. Thread nova, pai imutável.** Se `thread` já tem um `task.assign`:
  - com vínculo para o mesmo `parent`: aceito (reenvio da mesma subtarefa);
  - sem vínculo ou com outro: recusado (`parent_immutable`).
  Um `task.assign` posterior sem pai numa thread vinculada é aceito e mantém o vínculo. Como um vínculo só é criado no primeiro `task.assign` de uma thread, depois do do pai, não se formam ciclos.
- **R6. Profundidade.** Uma raiz (thread com `task.assign` e sem vínculo) tem profundidade 0. `profundidade(filho) = profundidade(pai) + 1`, no máximo **3** (`depth_exceeded`).
- **R7. Fan-out.** Um pai pode ter no máximo **8 filhos abertos** (`fanout_exceeded`). Um filho está aberto a menos que sua linha de claim esteja completada ou revogada, ou que ele não tenha linha de claim e todo `task.assign` nele tenha expirado pelo `ttl`. Um claim liberado está aberto, porque pode sofrer takeover.
- **R8.** Os limites são constantes na Fase A, não configuração (resolvida: constantes).
- **R9.** Os erros usam os códigos acima como prefixo do texto, como `lease_lost`, para que CLI, MCP e harness possam reconhecê-los.

### Complete

- **R10.** `Complete` numa thread com filhos abertos funciona como hoje e acrescenta `open_children` (lista ordenada de threads) ao resultado; vazio é omitido. A CLI imprime `warning: TKT-001 completed with 2 open children (TKT-001/api, TKT-001/ui)` no stderr. A mensagem `task.result` não muda.

### Visão de árvore

- **R11.** `tb bus tree <canal>` (humano, só CLI, como o `tb bus log`; funciona em canal expirado). Flags:
  - `--thread T`: só a subárvore com raiz em `T`;
  - `--all`: também raízes sem filhos (por padrão só aparecem árvores com pelo menos um vínculo, para não inundar canais do router);
  - `--limit N`: no máximo `N` raízes, mais novas primeiro (padrão 50);
  - `--json`.
- **R12.** Por nó: `thread`, `parent`, `depth`, `state`, `owner`, `epoch`, `assigned_by`, `assign_id`, `open_children`. `state` é, nesta ordem: `revoked`, `completed`, `released`, `claimed`, `expired` (sem claim, todo assign além do `ttl`), `assigned`. Não mostra payload, conteúdo do resultado, token nem a `session` do Agent Card.
- **R13.** Saída de texto, um nó por linha, recuo de dois espaços por nível:

  ```text
  TKT-001            claimed    agent:claude-main  epoch 1  open 1
    TKT-001/api      completed  agent:codex-1      epoch 1
    TKT-001/ui       claimed    agent:grok-1       epoch 2
  ```

### CLI e MCP

- **R14.** `tb bus send --parent T`. O `tb_bus_send` ganha uma string opcional `parent`. Nenhuma outra mudança no MCP; nenhuma ferramenta MCP para a árvore.
- **R15.** O `docs/bus-protocolo.md` documenta `parent` e R3–R10 nas regras do broker; o `docs/cli.md` ganha a flag `send --parent` e a linha de `tree`. Os dois com espelhos pt-BR.

### Store

- **R16.** Schema v3 → v4 na migração `IMMEDIATE` existente, de mão única, como no [bus-lease.md](../bus-lease.md):
  - `messages.parent TEXT`;
  - uma tabela nova:
    ```sql
    CREATE TABLE task_links (
      channel    TEXT NOT NULL REFERENCES channels(name),
      thread     TEXT NOT NULL,
      parent     TEXT NOT NULL,
      depth      INTEGER NOT NULL,
      assign_id  TEXT NOT NULL,
      created_at INTEGER NOT NULL,
      PRIMARY KEY (channel, thread)
    );
    CREATE INDEX task_links_parent ON task_links(channel, parent);
    ```
  - um índice parcial `messages_assign_thread ON messages(channel, thread) WHERE type = 'task.assign'`, para que as buscas "esta thread tem assign" não varram o log do canal;
  - Um store novo já nasce em v4. Threads existentes não têm vínculos (todas raízes). Um `tb` mais antigo recusa um store v4, então todo `tb` que serve um canal é atualizado junto.

## Modelo de ameaças

| Ameaça | Mitigação |
|--------|-----------|
| Um agente se diz subtarefa do trabalho de outro para ganhar legitimidade | R4: só o dono vivo do pai ou o router vinculam |
| Supervisor com prompt injetado faz fan-out sem limite | R6, R7: profundidade e fan-out aplicados pelo broker |
| Trocar o pai para esconder trabalho ou formar ciclos | R5: vínculo definido uma vez, no primeiro assign da thread |
| A visão de árvore vaza conteúdo | R12: sem payload, resultado ou segredos; CLI só do humano |
| Uma parte remota conduz a delegação por uma ponte A2A | ADR-020: a ponte é um agente comum; como `worker` não atribui, e como `supervisor` R4 e R7 continuam valendo |

Fronteira de confiança igual à do B1: mesmo usuário local; convites e identidade por conexão protegem contra injeção e roteamento errado, não contra um processo local hostil.

## Requisitos não funcionais

- Um send com `parent` acrescenta no máximo quatro buscas indexadas à transação existente.
- `tb bus tree` num canal com 10.000 mensagens e 500 vínculos: abaixo de 200 ms.

## Alternativas

Ver o [ADR-020](../adr/020-agent-hierarchy-a2a.md). Nesta spec: guardar o vínculo só na mensagem (sem tabela) foi rejeitado porque as checagens de profundidade e fan-out varreriam o log a cada envio.

## Questões abertas

Todas resolvidas pelo usuário em 2026-11-22.

- **Q1.** Construir a Fase A, ou declarar a hierarquia pronta com o router e os papéis do barramento? **Resolvida: construir a Fase A.**
- **Q2.** Manter `complete` com filhos abertos como aviso, ou recusá-lo? **Resolvida: só avisa** (R10). Recusar exigiria um caminho de revogação para o supervisor, que não tem CLI.
- **Q3.** Profundidade 3 e 8 filhos abertos como constantes, ou opções por canal? **Resolvida: constantes** (R8).
- O abandono do `tb agent` (ADR-020) está confirmado.

## Plano detalhado de implementação (um PR)

1. `internal/bus/store.go`: schema v4 (`messages.parent`, `task_links`), passo de migração 3 → 4; `SendInput.Parent`, `Message.Parent`; checagens R1, R3–R7 no `Send`; `insertMessage` e os selects levam `parent`.
2. `internal/bus/lease.go`: `CompleteResult.OpenChildren` (R10).
3. `internal/bus/tree.go` (novo): `Tree(channel, root string, all bool, limit int)` montando nós e estados (R11, R12).
4. `cmd/tb`: `bus send --parent`, `bus tree` (texto e `--json`), aviso no complete.
5. `internal/mcp`: `parent` no `tb_bus_send`.
6. Docs: `docs/bus-protocolo.md`, `docs/cli.md` e espelhos pt-BR; item do roadmap do README marcado; status desta spec.

### Plano de verificação

- Testes unitários em `internal/bus`: cada código de erro de R1–R7; duplicata idempotente com `parent`; reenvio para o mesmo pai (R5); reenvio sem pai mantém o vínculo; profundidade 3 aceita e 4 recusada; 8 abertos aceitos e o nono recusado, depois aceito quando um filho completa, é revogado ou expira sem claim; filho liberado continua aberto; supervisor com lease perdido recusado (R4); router vincula qualquer pai conhecido.
- Teste de migração: um store v3 com mensagens e claims abre em v4 sem vínculos; um store v4 é recusado por um leitor v3 (checagem de versão).
- `Complete` com filhos abertos os devolve e ainda completa.
- Testes de `tree`: estados, `--thread`, `--all`, `--limit`, formato JSON e uma checagem golden de que nenhum texto de payload aparece.
- O teste de docs da CLI (`TestDocsCLIExamplesMatchCobra`) passa com os exemplos novos; o teste de espelho de tradução passa.
- MCP: `tb_bus_send` com `parent` faz ida e volta; lista de ferramentas igual no resto.
- `tb stack verify guard` PASS; revisão em contexto novo.
