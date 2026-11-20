[🇺🇸 English](../bus-router.md) | 🇧🇷 Português

# Router no barramento — spec do B4

Status: **rascunho, rodada 2** (uma revisão em contexto novo aplicada; não implementado).

Especificação da etapa **B4** do [ADR-013](adr/013-barramento-multi-vendor.md) (§8): o router da Fase 3 ([ADR-009](adr/009-router-e-workers.md)) ganha um segundo tipo de worker, um agente registrado num canal do barramento. Apoia-se no protocolo ([bus-protocolo.md](bus-protocolo.md)), no lease ([bus-lease.md](bus-lease.md)) e no harness ([bus-harness.md](bus-harness.md), [bus-notify-discover.md](bus-notify-discover.md)).

## Problema

Hoje o `tb router run` roda um `router.agent_cmd` de uso único por ticket, no sandbox Landlock, confinado ao worktree do ticket. Um agente persistente de outro vendor (uma sessão do Codex ou do Claude Code acordada pelo `tb bus run`) não consegue pegar um ticket: não há como entregá-lo, saber que ele está trabalhando nem trazer o resultado de volta para o `tb merge`.

## Princípios

1. **O mesmo confinamento de um worker local.** Um agente do barramento que pega uma tarefa do router roda confinado ao worktree daquele ticket, pelo próprio harness. Um agente que pode escrever na raiz do repositório consegue reescrever filtros do `.git/config`, `.git/worktrees/`, tickets, estado do router e o `bus.db`, e aí nenhuma checagem do router valeria nada. Agentes não confinados são um opt-in explícito e documentado.
2. **O router continua sendo o único que escreve tickets e commits.** Um agente do barramento edita arquivos no worktree e informa um veredito. O router faz o commit, confere e registra.
3. **O mesmo contrato de resultado.** O `tb merge` continua lendo `Result.BaseCommit`/`HeadCommit` do repositório local e não muda.
4. **Toda tentativa é uma thread única e que pode ser fechada.** Threads nunca são reusadas, entre tentativas, execuções do router ou um board recriado, e o router consegue fechar uma de vez.
5. **A tabela de claims é a fonte da verdade.** O router lê claims e resultados por thread, nunca avançando um cursor que outro processo poderia compartilhar.
6. **A saída do agente não é confiável.** O veredito só decide se o router tenta o commit; são as checagens de integridade do router que decidem se o commit conta. Texto do agente vai para o log redigido do router, nunca para um campo de ticket ou um prompt.

## Configuração

1. O humano cria o canal e um convite `router`, e entra com o router pela CLI:

   ```bash
   tb bus channel create feature-x --owner leonardo
   tb bus invite feature-x --role router --name router
   tb bus join feature-x --invite <code> --name router --role router
   ```

2. Cada worker entra com `--role worker` e roda `tb bus run` com o adapter `harness` ou `exec` e `router_tasks: true` no arquivo de wake (abaixo).
3. A config do router:

   ```yaml
   router:
     max_workers: 2          # slots locais w1..wM; 0 = só barramento
     bus:
       channel: feature-x    # vazio = sem slots do barramento (comportamento atual)
       as: router            # a sessão CLI do router nesse canal
       slots: 2              # slots do barramento b1..bN (padrão 1 quando há canal)
       to: role:worker       # destinatário do task.assign, ou agent:<name> para fixar um agente
       claim_timeout: 5m     # um assign sem claim nesse prazo é retirado
       max_withdrawals: 3    # por ticket por execução; depois o ticket falha (unclaimed)
       trust: confined       # confined | unconfined (veja Confiança)
   ```

- `max_workers: 0` passa a ser válido só com `bus.channel` definido; o `router.New` deixa de trocar a config inteira pelos padrões quando `max_workers` é 0, e o `RouterConfig.Validate` aceita esse caso.
- Com `max_workers: 0`, o `preflight` deixa de exigir `agent_cmd` e Landlock (o router não roda agente). Com slots locais, os dois continuam obrigatórios.
- O `tb router run --check` (novo) valida a sessão, o papel `router`, o canal ativo, as contagens de slots e o `bus.to`. Avisa quando o canal tem mais de um agente com papel `router`, ou quando `bus.to` fixa um agente cujo card diz `max_concurrency` menor que `slots`. Não roda nada.

## Mudanças no barramento

O B4 precisa de três operações no store, todas com o member do router (papel `router`; `supervisor` também pode usar o `Revoke`):

| Operação | Faz |
|---|---|
| `Claim(channel, thread)` | lê uma linha de claim: dono, epoch, `claimed_at`, `released_at`, `completed_at`, `result_id`. Qualquer membro pode ler. |
| `Message(channel, id)` | lê uma mensagem pelo ID (para buscar o `task.result` que um claim aponta). |
| `Revoke(member, thread, reason)` | fecha uma thread de vez: sem claim, insere uma linha de claim do chamador, concluída sem resultado; com claim aberto, define `released_at` e `completed_at` e manda `task.status {"state":"revoked","thread":T,"reason":R}` para `agent:<dono>`. Um `claim` posterior falha com "already completed", um `complete` posterior com `lease_lost`. Só router e supervisor. |

- O `Revoke` é a cerca que faltava ao router: impede que o resultado de um dono desatualizado conte e faz um claim atrasado numa thread retirada falhar.
- Nenhuma outra regra do barramento muda. Qualquer um continua podendo mandar `task.status`, então quem consome precisa conferir o remetente (abaixo).

## Slots e agendamento

- O pool é `w1..wM` (locais), depois `b1..bN` (barramento). Os slots livres são preenchidos nessa ordem, então os locais mantêm a prioridade.
- `ValidWorkerID` passa a ser `^[wb]\d+$`; o `tb watchdog kill` aceita as duas formas, e a mensagem de erro lista as duas.
- Timeouts, `retry.max_attempts`, o timeout global e `slow_threshold` valem sem mudança. O timeout do ticket começa no claim, não no assign; a espera por um claim é limitada pelo `claim_timeout`.
- **A entrega direta não vale para slots do barramento no B4**: um dependente de um ticket concluído num slot do barramento vai para o `bus.to` como qualquer outro ticket (hoje as assinaturas são por slot, e um agente do barramento não é um slot).
- Cada tentativa no barramento roda na própria goroutine, como um worker local: consulta o store a cada tick (padrão 1s) e devolve `workerDone` para a goroutine do Run, que guarda todo o estado dos tickets. O cancelamento passa pelo `cancel()` do slot. O canal `done` tem tamanho `M + N`.

## Ciclo de vida de uma tentativa no barramento

### 1. Preparar

Como num worker local: ticket `running`, `Worker = bN`, `Attempts++`, worktree `.toolbelt/workspaces/<ID>` no branch `tb-workspace/<ID>` a partir da base, resetado antes.

A thread é `<ID>/<run>.<attempt>`, por exemplo `TKT-007/k3m9q2.2`:

- `<run>` são 6 caracteres base32 aleatórios escolhidos quando o router começa;
- ela é gravada no ticket como `Result.bus_thread` **no início**, para que o `recover` e a checagem do resultado a conheçam.

Um board recriado com IDs desde `TKT-001`, uma segunda execução ou uma tentativa devolvida à fila nunca reusam uma thread.

### 2. Assign

Um `task.assign` do router:

- destinatário: `bus.to`;
- thread: a thread da tentativa;
- chave de idempotência: `<thread>/assign`;
- payload: `router-task/0.1`, abaixo.

### 3. Claim

A goroutine consulta `Claim(thread)`. Um claim só conta quando tudo isto vale:

- o papel do dono é `worker`;
- quando `bus.to` é `agent:X`, o dono é `X`;
- `claimed_at` não é anterior ao `created_at` do assign;
- com `trust: confined`, o Agent Card do dono diz `sandbox: landlock` e `wake` é `harness` ou `exec`.

Um claim que falha em qualquer um deles é revogado (`Revoke`, motivo `claim_rejected`), registrado como `bus_claim_rejected`, e a thread é retirada como abaixo.

**Retirada.** Sem claim válido depois do `claim_timeout`, o router:

1. revoga a thread (motivo `unclaimed`);
2. devolve o ticket para `pending` sem gastar a tentativa (`Attempts--`, seguro porque a próxima tentativa ganha uma thread nova);
3. registra `bus_unclaimed`.

Depois de `max_withdrawals` retiradas numa execução, o ticket vai para `failed` com `error: unclaimed`.

### 4. Trabalho

A goroutine guarda o dono e o epoch do claim válido. A cada tick ela trata como **lease perdido**:

- um dono ou epoch diferente dos guardados;
- `released_at` preenchido;
- o dono sem estar vivo (lease do B2) durante um lease inteiro. Nesse caso o próprio router revoga a thread (motivo `lease_expired`), em vez de esperar um humano rodar o `tb bus watchdog`.

Com o lease perdido, a tentativa falha com `bus_lease_lost`.

Timeout e cancelamento:

- no `slow_threshold`, o router registra `worker_slow`;
- no timeout, revoga (motivo `timeout`) e falha a tentativa (`worker_stuck`);
- um ticket alterado fora do router, ou um `tb watchdog kill bN`, revoga (motivo `canceled`) e libera o slot. A saída do kill diz que o processo remoto não é parado, só cercado.

### 5. Resultado

O agente conclui com:

```bash
tb bus complete TKT-007/k3m9q2.2 --epoch E --result '{"verdict":"PASS"}'
```

A goroutine vê `completed_at` preenchido no claim, com o dono e o epoch guardados. Ela busca `Message(result_id)` e confere que o `from` é o dono e que a thread é a da tentativa.

- **O primeiro evento terminal na ordem do store vence.** Um resultado concluído antes do revoke do router conta, mesmo que o router o perceba depois (como um worker local que saiu logo antes do cancelamento).
- **`verdict: "PASS"`.** Antes do commit, o router confere:
  - `symbolic-ref HEAD` é `refs/heads/tb-workspace/<ID>`;
  - `rev-parse HEAD` ainda é a base: o agente não fez commit, não trocou de branch nem desanexou o HEAD;
  - a ref do branch ainda é a base.

  Depois roda o mesmo `commitWorkspace` de um worker local (`verifyWorkspace`, sem `.git` aninhado, sem gitlinks nem `.gitmodules`, `git add -A` endurecido, commit `tb: <ID> <title>`) e confere que o novo HEAD tem exatamente um pai, a base.
  - Qualquer divergência falha a tentativa como erro de integridade, escalado sem nova tentativa, como hoje.
  - Um PASS sem mudanças fica `done` com `HeadCommit == BaseCommit`, como num worker local; o `tb merge` trata isso como no-op.
- **`verdict: "FAIL"`, ou qualquer outro valor ou forma:** a tentativa falha (`bus_verdict_fail`) e é refeita enquanto houver tentativas.

### 6. Nova tentativa, desligamento, reinício

- **Nova tentativa.** Uma nova tentativa reseta o worktree e abre uma thread nova. Um agente desatualizado e confinado não consegue escrever no worktree novo: a regra Landlock dele aponta para o diretório antigo, e o reset o recria.
- **Desligamento e timeout global.** Cada tentativa do barramento em andamento é revogada (motivo `shutdown`) e devolvida para `pending` com `Attempts--`, como os workers locais.
- **Reinício.** O `recover` revoga a `bus_thread` de todo ticket do barramento em `running` antes de devolvê-lo para `pending`.
- **Canal expirado.** Um canal depois do `--ttl` recusa escritas, então:
  - erros de escrita no barramento são registrados e nunca param o laço;
  - as tentativas do barramento em andamento voltam para `pending` na hora;
  - os slots do barramento ficam parados com um alerta `bus_channel_expired`, e os slots locais seguem rodando.

### Payload `router-task/0.1`

```json
{
  "v": "router-task/0.1",
  "ticket": "TKT-007",
  "thread": "TKT-007/k3m9q2.2",
  "attempt": 2,
  "title": "Add retry to the webhook client",
  "workspace": "/home/leonardo/src/app/.toolbelt/workspaces/TKT-007",
  "ticket_file": "/home/leonardo/src/app/.toolbelt/tickets/TKT-007.yaml",
  "branch": "tb-workspace/TKT-007",
  "base": "a1b2c3d4e5f6...",
  "claim_by": "2026-11-20T18:05:00Z",
  "timeout": "10m"
}
```

- Descrição, mandato e critérios de aceite são lidos do `ticket_file`; não são copiados para a mensagem.
- `title` é o título do ticket como está gravado. Ele pode ter sido escrito por um modelo planejador, então é tratado como dado. Só entra na mensagem de commit depois de tirados os caracteres de controle, como hoje.

### Payload do resultado

```json
{"verdict": "PASS", "summary": "opcional, no máximo 500 bytes"}
```

- `summary` é cortado em 500 bytes, sem caracteres de controle nem sequências ANSI, redigido de novo e gravado só em `.toolbelt/router/logs/<ID>.log`.
- Qualquer outro campo é ignorado. Um `commit`, `head` ou `branch` vindo do agente nunca é usado.

## Lado do worker: `router_tasks`

O harness ganha uma opção no arquivo de wake, `router_tasks: true` (adapters `harness` e `exec`; recusada com `notify`). Com ela, um `task.assign` só é tratado como tarefa do router quando tudo isto vale:

- o papel do remetente é `router` (consultado no store, não tirado do payload);
- o payload é `router-task/0.1`;
- `workspace` é exatamente `<root>/.toolbelt/workspaces/<ticket>` e existe;
- `ticket_file` é exatamente `<root>/.toolbelt/tickets/<ticket>.yaml`;
- `thread` é igual à thread da mensagem.

Qualquer outra coisa é uma mensagem comum: acorda como sempre, sem workspace.

Para uma tarefa válida do router, o harness acorda o agente **uma vez por tarefa**, com:

- **workdir** = o `workspace` do payload, o único caminho com escrita além do diretório temporário do próprio agente;
- permissão **só de leitura** no `ticket_file`;
- **nenhuma permissão** na raiz do repositório, em `.git` ou em `.toolbelt` (o arquivo `.git` do worktree continua legível, como para workers locais);
- um **prompt fixo** que nomeia o ticket e a thread e manda o agente:
  - fazer claim da thread (`tb_bus_claim`);
  - trabalhar só no diretório atual;
  - não fazer commit;
  - concluir com `{"verdict":"PASS"}` ou `{"verdict":"FAIL","summary":"..."}`, passando o epoch do claim.

O harness também:

- só age sobre `task.status` `revoked`/`canceled` quando o papel do remetente é `router` ou `supervisor`, e ignora os demais;
- registra `router_task` na auditoria (ticket e thread, sem payload);
- inclui `task.status` nos tipos que acordam para tarefas do router, para que um revoke acorde o agente e o prompt mande ele parar.

O router ignora todo `task.status` exceto os próprios revokes (ele lê claims, não mensagens de status). O status `wake_failed` do harness também é ignorado; o timeout do claim cobre esse caso.

## Confiança

- **`trust: confined` (padrão).**
  - Só são aceitos claims de agentes cujo card declara `sandbox: landlock` e um wake `harness` ou `exec`.
  - A declaração é do próprio agente: o router não consegue inspecionar outro processo. Essa é a fronteira de confiança do B1 (um processo hostil do mesmo usuário fica fora do escopo); o que o confinamento protege é contra o modelo do agente escrever fora da tarefa.
  - Com `router_tasks`, é o harness que a impõe.
- **`trust: unconfined`** aceita qualquer claim de `worker`, incluindo agentes de GUI (`notify`), harnesses com `--allow-unsandboxed` e harnesses sem `router_tasks`.
  - O `tb router run` imprime um aviso ao iniciar e em cada claim assim (`bus_unconfined_claim`).
  - Nesse modo um agente pode escrever na raiz do repositório. As checagens de integridade do router, os tickets e as garantias do `tb merge` não valem contra ele. Use só com agentes que você deixaria rodar `git` no repositório.
- **Mandato.** Como hoje, o router não impõe `mandate.scope`/`forbidden` nos caminhos; quem faz isso é o `tb stack` (`guard`, `review --ticket`), antes do merge.
- **Um host.** Os caminhos de workspace são locais; um agente do barramento em outra máquina não consegue pegar uma tarefa do router.

## Estado, watchdog e board

- O `state.json` ganha, por slot do barramento, `thread`, `agent` (depois do claim), `claimed_at` e `phase` (`assigned`, `claimed`, `committing`).
- `tb watchdog status` e `tb board` os mostram para os tickets do barramento em andamento.
- Novos tipos de alerta: `bus_unclaimed`, `bus_claim_rejected`, `bus_lease_lost`, `bus_verdict_fail`, `bus_unconfined_claim`, `bus_channel_expired`, `bus_write_failed`.
- O `tb router run --dry-run` mostra qual slot (`wN` ou `bN`) cada ticket pronto pegaria.
- `Result` ganha `bus_thread` (definido no início), `bus_agent` e `bus_epoch` (definidos no `done`, a partir da linha de claim, nunca do payload).

## Fora do B4

- Entrega direta para slots do barramento, e reservas por agente.
- L2 (triagem e resumos por modelo local), a escada de escalonamento por tier e a revisão entre vendors (ADR-013 §5).
- Agentes em outro host.
- Escolher um agente por capacidade ou tier a partir do card.
- Impor o mandato nos caminhos de arquivo.
- Tarefas do router no preset `claude` com um workdir diferente do payload (o `harness.workdir` do preset é ignorado para tarefas do router).

## Critérios de aceite

1. **Caminho feliz.** Só barramento (`max_workers: 0`), com um worker falso e confinado que faz claim, edita o worktree e conclui com `PASS`:
   - o ticket termina `done`, com um único commit `tb: <ID> <title>` em `tb-workspace/<ID>` cujo único pai é a base;
   - `bus_thread`, `bus_agent` e `bus_epoch` estão preenchidos;
   - o `tb merge <ID>` aplica o commit sem mudança.
2. **Assign.** O payload do `task.assign` segue o `router-task/0.1` e vai para o `bus.to` com a thread `<ID>/<run>.<attempt>`. Ele não leva descrição, mandato nem texto de aceite.
3. **Nova tentativa.** `FAIL` ou um resultado malformado gera nova tentativa numa thread nova com o worktree resetado; depois de `max_attempts` o ticket fica `failed`.
4. **Agente faz commit.** Um agente que faz commit sozinho, troca de branch ou desanexa o HEAD falha na checagem antes do commit e escala sem nova tentativa. Um commit do agente com um gitlink nunca chega a `done`. Um resultado que cita outro SHA não tem efeito.
5. **Retirada.** Sem claim dentro do `claim_timeout`:
   - a thread é revogada, e um claim posterior nela falha com "already completed";
   - o ticket volta para `pending` sem gastar a tentativa, e `bus_unclaimed` é registrado;
   - depois de `max_withdrawals` o ticket fica `failed`.
6. **Claims inválidos.** Cada um é revogado com `bus_claim_rejected`:
   - um claim de um reviewer;
   - com `bus.to: agent:a`, um claim do agente `b`;
   - um claim feito antes do assign;
   - em `confined`, um claim de um agente cujo card diz `sandbox: none`.
7. **Lease perdido.** Em cada caso a tentativa falha com `bus_lease_lost`, e o `complete` posterior do dono devolve `lease_lost`:
   - um dono calado além do lease, sem nenhum humano rodar o watchdog;
   - uma tomada por outro worker.
8. **Timeout.** `worker_slow` no limiar; no timeout, revoke e `worker_stuck`. Um `PASS` gravado antes do revoke continua contando.
9. **Resultados velhos.** Nenhum destes marca um ticket como `done`:
   - um `task.result` de uma execução anterior do router;
   - um de um ticket de mesmo nome num board recriado;
   - um de uma thread revogada.
10. **Reinício e desligamento.**
    - Reiniciar o router revoga a thread aberta dos tickets do barramento em andamento e os devolve para `pending`.
    - O desligamento e o timeout global terminam sem travar enquanto os slots do barramento estão ocupados.
11. **Sem cursor.** Um humano rodando `tb bus recv --as router` durante uma execução não muda o que o router vê.
12. **Slots mistos.** Slots locais e do barramento rodam lado a lado; os locais são preenchidos primeiro.
13. **Canal expirado no meio de uma tentativa.** As tentativas do barramento voltam para `pending`, um alerta `bus_channel_expired`, e os slots locais seguem trabalhando.
14. **`--check` e `preflight`.**
    - O `--check` recusa sessão ausente, papel que não é router e canal expirado, e avisa quando há um segundo agente com papel router.
    - O `preflight` recusa `max_workers: 0` sem `bus.channel`, e o aceita sem `agent_cmd` e sem Landlock quando `bus.channel` está definido.
15. **Summary.** `summary` só aparece no log do router, cortado, limpo e redigido, nunca no YAML do ticket.
16. **`router_tasks` do harness.**
    - Uma tarefa válida do router roda o filho com o workspace como único caminho com escrita, mais um `ticket_file` só de leitura; num host com Landlock, uma escrita na raiz do repositório ou no `.git/config` falha.
    - Um `router-task` mandado por um supervisor, ou com `workspace` fora de `.toolbelt/workspaces/<ticket>`, acorda como mensagem comum, sem workspace.
17. **Status forjado.** Um `task.status` `revoked` mandado por um worker para outro é ignorado pelo harness desse worker, e o router nunca age sobre ele.
18. **Não confinado.** Em `trust: unconfined`, o claim de um agente `notify` é aceito, com aviso ao iniciar e alerta `bus_unconfined_claim`.
19. **Kill do watchdog.** `tb watchdog kill b1` revoga a tentativa, libera o slot e informa que o processo remoto não é parado.
