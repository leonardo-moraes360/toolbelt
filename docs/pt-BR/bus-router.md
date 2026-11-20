[🇺🇸 English](../bus-router.md) | 🇧🇷 Português

# Router no barramento — spec do B4

Status: **implementado** (B4a e B4b; spec na rodada 3, depois de duas revisões em contexto novo). Onde o código é mais simples que este texto, a nota *Neste corte* diz isso.

Especificação da etapa **B4** do [ADR-013](adr/013-barramento-multi-vendor.md) (§8): o router da Fase 3 ([ADR-009](adr/009-router-e-workers.md)) ganha um segundo tipo de worker, um agente registrado num canal do barramento. Apoia-se no protocolo ([bus-protocolo.md](bus-protocolo.md)), no lease ([bus-lease.md](bus-lease.md)) e no harness ([bus-harness.md](bus-harness.md), [bus-notify-discover.md](bus-notify-discover.md)).

O B4 sai em duas partes, cada uma testável sozinha:

- **B4a:** operações do barramento e o modo `router_tasks` do harness. Dá para testar com um router falso.
- **B4b:** slots do barramento no `tb router run`.

## Problema

Hoje o `tb router run` roda um `router.agent_cmd` de uso único por ticket, no sandbox Landlock, confinado ao worktree do ticket. Um agente persistente de outro vendor (uma sessão do Codex ou do Claude Code acordada pelo `tb bus run`) não consegue pegar um ticket: não há como entregá-lo, saber que ele está trabalhando nem trazer o resultado de volta para o `tb merge`.

## Princípios

1. **O mesmo confinamento de um worker local.** O modelo que trabalha numa tarefa do router roda confinado ao worktree daquele ticket. Um agente que pode escrever na raiz do repositório consegue reescrever filtros do `.git/config`, `.git/worktrees/`, tickets, estado do router e o `bus.db`, e aí nenhuma checagem do router valeria nada. O B4 não tem modo não confinado.
2. **O modelo nunca toca o barramento numa tarefa do router.** O processo do harness, fora do sandbox, faz o claim, mantém o lease e conclui. O filho confinado só edita arquivos e escreve um arquivo de veredito. Um filho que conseguisse abrir o `bus.db` poderia forjar claims, resultados e revokes.
3. **O router continua sendo o único que escreve tickets e commits.** Ele só faz o commit do worktree depois que o harness concluiu, o que só acontece depois que o grupo de processos do filho terminou; então nada escreve enquanto ele faz o commit.
4. **O mesmo contrato de resultado.** O `tb merge` continua lendo `Result.BaseCommit`/`HeadCommit` do repositório local e não muda.
5. **Toda tentativa é uma thread única e que pode ser fechada.** Threads nunca são reusadas, e o router consegue fechar uma de vez com uma única transação que também diz se um resultado chegou antes.
6. **A tabela de claims é a fonte da verdade.** O router lê claims e resultados por thread, nunca avançando um cursor.
7. **A saída do agente não é confiável.** O veredito só decide se o router tenta o commit; são as checagens de integridade do router que decidem se o commit conta. Texto do agente vai só para o log redigido do router.

## Parte B4a: barramento e harness

### Operações do barramento

O schema do store vai da v2 para a v3: `claims` ganha `revoked_at`, `revoked_by`, `revoke_reason` e `confinement` (texto, escrito só pelo claim do harness abaixo). A migração segue as regras da v1 → v2: uma transação IMMEDIATE, upgrade de mão única.

| Operação | Quem | Faz |
|---|---|---|
| `Claim(channel, thread)` | qualquer membro | Lê uma linha de claim: dono, epoch, `claimed_at`, `released_at`, `completed_at`, `result_id`, `revoked_at`, `revoke_reason`, `confinement`. Funciona num canal expirado (é leitura). |
| `Message(channel, id)` | qualquer membro | Lê uma mensagem pelo ID. Funciona num canal expirado. |
| `Revoke(member, thread, reason)` | `router` ou `supervisor` | Uma transação IMMEDIATE que devolve o estado final do claim. |
| `ClaimConfined(member, thread, marker)` | o harness (API interna, sem flag de CLI nem tool MCP) | Como `ClaimThread`, e grava o `marker` em `confinement`. |

**Casos do `Revoke`:**

- A thread foi **concluída** com um resultado: nada muda, e ele devolve `completed` com o `result_id`. Quem chama usa isso como decisão: um resultado que chegou antes vence.
- A thread está **aberta** (com claim, não concluída): define `revoked_at`, `revoked_by`, `revoke_reason` e `released_at`, e devolve `revoked`.
- A thread está **sem claim**: insere uma linha do chamador com `revoked_at` definido, e devolve `revoked`.
- A thread **já estava revogada**: nada muda; ele devolve `revoked`.

**Depois de um revoke:**

- `ClaimThread` falha com `thread revoked` (conferido antes de "already completed").
- `Complete` falha com `lease_lost` (o revoke é conferido antes da conclusão).
- `Heartbeat` lista a thread em `lost`, com o motivo.

**O `marker`** é um objeto JSON pequeno, `{"router_tasks":true,"landlock":true}`. O router (B4b) só aceita claims que o trazem.

- Um membro que chama o `tb bus claim` ou o `tb_bus_claim` comuns não ganha marker, e uma thread que ele já segura assim nunca ganha um: o `ClaimConfined` nela falha.
- O marker diz o que o processo do harness do `tb` impôs; não é uma credencial. Dentro da fronteira de confiança do B1 (um processo hostil do mesmo usuário fica fora do escopo), ele responde se um harness mal configurado pegou a tarefa.

### `router_tasks` do harness

O arquivo de wake ganha:

```yaml
router_tasks:
  from: router        # o único agente cujos assigns de tarefa do router são aceitos
  timeout: 30m        # teto por tarefa; vale o timeout do payload se for menor
```

- `router_tasks` é aceito com `adapter: harness` ou `exec`, e recusado com `notify`.
- O `--check` o recusa sem Landlock: `--allow-unsandboxed` é incompatível com tarefas do router.

**Quais mensagens são tarefas do router.** Um `task.assign` só é tratado como tarefa do router quando tudo isto vale:

- o remetente é `agent:<from>`, e o papel dele no store é `router`;
- o payload é `router-task/0.1`;
- `ticket` casa com `^TKT-\d{3,}$`;
- `workspace` resolve (seguindo todos os symlinks) para exatamente `<raiz canônica>/.toolbelt/workspaces/<ticket>`, um diretório de verdade;
- `ticket_file` resolve para exatamente `<raiz canônica>/.toolbelt/tickets/<ticket>.yaml`, um arquivo regular;
- esses caminhos são conferidos de novo logo antes de o filho começar, e o filho recebe os canônicos;
- `thread` é igual à thread da mensagem, e o `Claim(thread)` mostra que ela não está concluída nem revogada.

Qualquer outro `task.assign` é uma mensagem comum.

**Um worker dedicado a tarefas.** Um harness com `router_tasks` não roda wakes comuns: as mensagens comuns ficam na fila, intocadas. Senão os wakes comuns do agente rodariam com escrita na raiz do repositório e com tools do barramento, e um `recv` num deles poderia consumir uma tarefa. O harness lê as mensagens `task.assign` a partir do próprio cursor de tarefas (`task_seq` no estado, avançado antes de cada claim), em ordem e no máximo 50 por tick. Um reinício nunca trata uma tarefa duas vezes, e uma enxurrada de assigns inválidos não consegue deixar um válido esperando para sempre. O `wake_on` não vale para tarefas do router.

**Como uma tarefa é tratada:**

1. **Claim.** `ClaimConfined(thread, marker)`. Se o claim falhar (outro worker ganhou, ou a thread foi revogada), o harness registra `router_task_lost` e para: sem filho, sem acesso ao workspace.
2. **Preparar.**
   - Copia o `ticket_file` para `<tmp do agente>/task/ticket.yaml`. O arquivo vivo não recebe permissão, porque as escritas de ticket o renomeiam.
   - Cria um diretório temporário novo por tarefa; o próprio filho escreve `<dir da tarefa>/verdict.json`.
   - Recusa rodar sem Landlock, seja como for que o harness foi aberto (`router_task_refused`).
3. **Rodar o filho**, confinado. As permissões dele:
   - **escrita:** o workspace (o workdir dele), o diretório da tarefa (com o próprio tmp) e as entradas de `sandbox.write`;
   - **leitura:** `<root>/.git` (para `git status` e `git diff` funcionarem; o índice pode ficar sem escrita), a cópia da tarefa e as entradas de `sandbox.read`;
   - **recusadas, no `--check` e na hora do wake:** qualquer entrada de `sandbox.write` igual a `<root>`, `<root>/.git` ou `<root>/.toolbelt`, que contenha um deles ou que fique dentro de um deles; e qualquer permissão de leitura (`sandbox.read` ou os diretórios do binário do adapter) que contenha `<root>/.toolbelt` ou fique dentro dele. Os caminhos são comparados depois de resolver symlinks dos dois lados;
   - **sem acesso ao barramento:** o preset `claude` recebe uma config MCP vazia, `router_tasks.allowed_tools` (padrão `Read`, `Edit`, `Write`, `Glob`, `Grep`; `mcp__*` recusado) e `--disallowedTools Bash,WebFetch,WebSearch,Task`, e o `exec` não recebe token de sessão;
   - **ambiente:** `TB_TASK_TICKET`, `TB_TASK_TICKET_FILE` (a cópia), `TB_TASK_VERDICT` e `TMPDIR` (o tmp da tarefa);
   - **sessão nova:** o preset `claude` começa uma sessão nova por tarefa, sem `--resume`, para que o contexto de um ticket nunca vaze para o próximo;
   - **timeout:** o menor entre `router_tasks.timeout`, o `timeout` do payload e o `timeout` do adapter (que tem padrão de 1h quando `router_tasks` está definido).
4. **Prompt fixo** (`claude`), ou um packet com `reason: "router_task"` e um objeto `task` `{ticket, thread, workspace, ticket_copy, verdict_file}` (`exec`). Ele manda o agente:
   - trabalhar só no diretório atual;
   - ler o ticket da cópia;
   - não fazer commit (o git consegue ler, mas o repositório não tem escrita);
   - escrever `{"verdict":"PASS"}` ou `{"verdict":"FAIL","summary":"..."}` no `verdict_file` e sair.
5. **Vigiar.** Enquanto o filho roda, uma goroutine vigia (ao lado da do heartbeat) consulta `Claim(thread)` a cada segundo. Se vir um revoke, uma liberação, ou um dono ou epoch diferente do harness, ela mata o grupo de processos do filho e registra `router_task_revoked`.
6. **Concluir.** Depois que o grupo de processos do filho terminou (o harness mata o que restar do grupo), o harness lê o `verdict_file`. Ele precisa ser um arquivo regular de até 4 KiB com um único objeto JSON.
   - O harness chama `Complete(thread, epoch, verdict)`.
   - Só `PASS` e `FAIL` são aceitos; o summary é cortado em 500 bytes num limite de caractere e redigido.
   - Um veredito ausente, grande demais, em symlink ou malformado, uma queda ou um timeout concluem com `{"verdict":"FAIL","summary":"<motivo>"}`. Aqui `<motivo>` é texto fixo do harness, não a saída do filho.
   - Um `lease_lost` no complete (revogado no meio-tempo) é registrado, e nada mais acontece.
   - No desligamento do harness o filho é parado e nada é concluído (`router_task_canceled`): o lease acaba ou o router revoga.
7. **Auditoria.** `router_task_start`, `router_task_ok`, `router_task_failed`, `router_task_lost`, `router_task_revoked`, `router_task_canceled` e `router_task_refused`, com ticket, thread e epoch. Nunca o payload nem o summary do veredito.

- O heartbeat do harness mantém o lease do claim enquanto o filho roda, como no B2.
- Mensagens `task.status` não entram aqui: os revokes são lidos da linha de claim.
- **Landlock e novas tentativas.** A regra de um filho fica presa ao inode do diretório do workspace no momento do wake. O router revoga antes de resetar um worktree, e a vigia mata o filho no revoke, então nenhum filho de uma tentativa antiga está rodando quando o diretório é recriado. Um filho que por acaso continue vivo mantém a regra no inode antigo e não consegue escrever no novo.

## Parte B4b: slots do barramento no router

### Configuração

As configurações do barramento vêm só da **config confiável**: a config global ou `-c`, pelo `config.LoadTrusted`, nunca o `.toolbelt/config.yaml` do repositório. Um agente que pode escrever no repositório não consegue redirecionar o router.

```yaml
router:
  max_workers: 2          # slots locais w1..wM; 0 = só barramento
  bus:
    channel: feature-x    # vazio = sem slots do barramento (comportamento atual)
    as: router            # a sessão CLI do router nesse canal
    slots: 2              # slots do barramento b1..bN (padrão 1 quando há canal)
    to: role:worker       # destinatário do task.assign, ou agent:<name> para fixar um agente
    claim_timeout: 5m     # uma tentativa sem claim válido nesse prazo falha (unclaimed)
```

- **O `RouterConfig` guarda o bloco do barramento.** O `Validate` aceita `max_workers: 0` só com `bus.channel`. O `router.New` deixa de trocar a config inteira quando `max_workers` é 0.
- **O `--workers`** não expressa "só barramento" (ele só sobrescreve quando é maior que 0); isso fica na config.
- **O `preflight`** com `max_workers: 0` deixa de exigir `agent_cmd` e Landlock (o router não roda agente); com slots locais, os dois continuam obrigatórios.
- **O `tb router run --check` (novo)** recusa cada um destes, e não roda nada:
  - sessão ausente, ou com papel que não é `router`;
  - canal expirado;
  - mais de um membro com papel `router` no canal (dois routers brigariam pelos mesmos tickets);
  - um `bus.to` inválido.

### Slots e agendamento

- O pool é `w1..wM` (locais), depois `b1..bN` (barramento). Os slots livres são preenchidos nessa ordem.
- `ValidWorkerID` passa a ser `^[wb]\d+$`; o `tb watchdog kill` aceita as duas formas, e a mensagem de erro lista as duas.
- Cada tentativa do barramento roda na própria goroutine. Ela consulta o store a cada segundo e devolve `workerDone` para a goroutine do Run, que guarda todo o estado dos tickets. O cancelamento passa pelo `cancel()` do slot. O canal `done` tem tamanho `M + N`.
- **Timeouts.** O timeout do ticket começa no claim: o slot ganha `claimedAt`, usado pelo watchdog do router e pelo `tb watchdog status`. Antes do claim, o `claim_timeout` limita a espera.
- A entrega direta não vale para slots do barramento: os dependentes vão para o `bus.to`.

### Ciclo de vida de uma tentativa no barramento

**1. Preparar.** Como num worker local: ticket `running`, `Worker = bN`, `Attempts++`, worktree resetado e criado a partir da base.

A thread é `<ID>/<rand>.<attempt>`, por exemplo `TKT-007/k3m9q2.2`, onde `<rand>` são 6 caracteres base32 aleatórios por thread. Ela é gravada num campo novo do ticket, `bus_thread`, fora do `Result`:

- escrito no início;
- mantido quando a tentativa volta para a fila;
- limpo só depois que o router confirmou que a thread foi concluída ou revogada.

**2. Assign.** Um `task.assign` do router:

- destinatário: `bus.to`;
- thread: a thread da tentativa;
- chave de idempotência: `<thread>/assign`;
- `ttl`: `claim_timeout`;
- payload: `router-task/0.1`, abaixo.

**3. Claim.** A goroutine consulta `Claim(thread)`. Um claim só é válido quando tudo isto vale:

- o papel do dono é `worker`;
- o dono é o destinatário, quando `bus.to` é `agent:X`;
- `claimed_at` não é anterior ao `created_at` do assign;
- `confinement` traz o marker de router tasks.

Um claim inválido é revogado na hora (motivo `claim_rejected`) e registrado como `bus_claim_rejected`. O router então manda o assign de novo numa thread nova, dentro da mesma tentativa; o prazo de claim da tentativa não recomeça.

Sem claim válido até o prazo, o router revoga (motivo `unclaimed`) e falha a tentativa com `error: unclaimed`. A tentativa é gasta, então o `retry.max_attempts` limita um worker ausente.

**4. Trabalho.** A goroutine guarda o dono e o epoch do claim válido. A cada tick, ela revoga a thread e falha a tentativa com `bus_lease_lost` em qualquer um destes casos:

- um dono ou epoch diferente dos guardados;
- `released_at` preenchido;
- o dono sem estar vivo pela definição do B2 (`now >= last_seen + lease`).

Timeout e cancelamento também revogam:

| Evento | Motivo do revoke | Tentativa |
|---|---|---|
| `slow_threshold` atingido | (sem revoke) | registra `worker_slow` |
| timeout do ticket | `timeout` | falha com `worker_stuck` |
| ticket alterado fora do router, ou `tb watchdog kill bN` | `canceled` | slot liberado |

A saída do kill diz que o filho do harness é parado pela vigia do harness, não pelo router. *Neste corte* todo cancelamento (timeout, kill, ticket alterado, desligamento) revoga com o motivo `canceled`; lease perdido usa `lease_lost`, uma retirada `unclaimed` e um claim recusado `claim_rejected`.

Em todos os casos o router age conforme o que o `Revoke` devolve. `completed` quer dizer que um resultado chegou antes, e o router vai para o passo 5 com ele.

**5. Resultado.** A goroutine vê `completed_at` com o dono e o epoch guardados, ou recebe `completed` de um `Revoke`. Ela busca `Message(result_id)` e confere que o `from` é o dono e que a thread é a da tentativa.

- **`verdict: "PASS"`.** O router confere:
  - `symbolic-ref HEAD` é `refs/heads/tb-workspace/<ID>`;
  - `rev-parse HEAD` é a base;
  - a ref do branch é a base.

  Depois roda o mesmo `commitWorkspace` de um worker local. Quando havia algo para commitar, o novo HEAD precisa ter exatamente um pai, a base. Um PASS sem mudanças fica `done` com `HeadCommit == BaseCommit`, um no-op para o `tb merge`.
  - Qualquer divergência é um erro de integridade, escalado sem nova tentativa, como hoje.
  - Um cancelamento que chega durante o commit é ignorado até ele terminar.
- **`verdict: "FAIL"`, ou qualquer outro valor ou forma:** a tentativa falha (`bus_verdict_fail`) e é refeita enquanto houver tentativas.
- **O assunto do commit** é `tb: <ID> <title>`, com os caracteres de controle tirados do título. Isso é novo para os dois tipos de worker.

**6. Nova tentativa, desligamento, reinício, expiração.**

- **Nova tentativa:** a thread anterior é revogada (ou confirmada como concluída) antes do reset do worktree. Um reset que falha (`RemoveAll` bloqueado) gera `workspace_reset_failed` e falha o ticket, em vez de ficar em laço.
- **Desligamento e timeout global:** cada tentativa do barramento é revogada (motivo `shutdown`). Se o `Revoke` devolver `completed`, o resultado é processado; senão a tentativa volta para `pending` com `Attempts--`. O revoke é limitado pelo busy timeout de 5 s do store.
- **Reinício:** o `recover` revoga a `bus_thread` de todo ticket em `running`. *Neste corte* um resultado que uma execução anterior deixou concluído não é aceito: o ticket volta para `pending` e a próxima tentativa refaz o trabalho. Se o revoke falhar (canal expirado, store ocupado), o ticket volta para `pending`, a `bus_thread` é mantida, `bus_write_failed` é registrado e o revoke é tentado de novo no próximo início, antes de o ticket ser atribuído outra vez.
- **Canal expirado:** `Claim` e `Message` continuam funcionando.
  - Uma tentativa em andamento cujo claim mostra um resultado é aceita.
  - As outras voltam para `pending` com a `bus_thread` mantida.
  - Os slots do barramento ficam parados com um alerta `bus_channel_expired`, e os slots locais seguem rodando.
  - Erros de escrita no barramento são registrados (`bus_write_failed`) e nunca param o laço.

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
  "timeout": "10m"
}
```

- Descrição, mandato e critérios de aceite são lidos do arquivo do ticket (pela cópia do harness). Não estão na mensagem.
- `title` é o título gravado no ticket. Ele pode vir de um modelo planejador, então é dado.

### Payload do resultado

O harness o escreve a partir do arquivo de veredito:

```json
{"verdict": "PASS", "summary": "opcional, no máximo 500 bytes"}
```

- O router corta o `summary` em 500 bytes e tira caracteres de controle e sequências ANSI.
- Depois aplica as regras de redação do harness (compartilhadas a partir do `busrun`) e grava o summary só em `.toolbelt/router/logs/<ID>.log`.
- Outros campos são ignorados. Um `commit`, `head` ou `branch` vindo do agente nunca é usado.

### Estado

- `Ticket.bus_thread`, como acima.
- `Result` ganha `bus_agent` e `bus_epoch`, definidos no `done` a partir da linha de claim e levados no `workerDone`.
- O `state.json` ganha `thread`, `agent`, `claimed_at` e `phase` (`assigned`, `claimed`, `committing`) por slot do barramento. O `tb watchdog status` os mostra.
- Novos tipos de alerta: `bus_unclaimed`, `bus_claim_rejected`, `bus_lease_lost`, `bus_verdict_fail`, `bus_channel_expired`, `bus_write_failed`, `workspace_reset_failed`.
- O `tb router run --dry-run` mostra qual slot (`wN` ou `bN`) cada ticket pronto pegaria.

## Confiança e limites

- **O confinamento só vale enquanto nenhum membro com escrita na raiz do repositório estiver no canal.** Um agente no modo padrão do `harness` (workdir = raiz do repositório) continua podendo editar `.git`, tickets ou o `bus.db` direto, faça o router o que fizer. Rode esses agentes (um reviewer, um chat) em outro canal, ou confine-os também.
- **O marker** diz que o harness do `tb` impôs o confinamento, que é o que uma configuração errada quebraria. Um processo hostil do mesmo usuário fica fora do escopo, como no B1.
- **`~/.claude` em `sandbox.write`** (de que a CLI do `claude` precisa) deixa o modelo confinado escrever as configurações do Claude do usuário, inclusive hooks que rodam depois fora de qualquer sandbox. Prefira liberar só `~/.claude/projects` e o arquivo de credenciais, e trate uma permissão mais ampla como confiar ao modelo a configuração do Claude do usuário.
- **Mandato.** O router não impõe `mandate.scope`/`forbidden` nos caminhos. Quem faz isso é o `tb stack` (`guard`, `review --ticket`), antes do merge.
- **Um host.** Os caminhos de workspace são locais.

## Fora do B4

- Um modo `trust: unconfined` (agentes de GUI ou sem sandbox pegando tarefas do router).
- Entrega direta para slots do barramento, e reservas por agente.
- Um orçamento de retiradas separado do `retry.max_attempts`.
- Colunas do barramento no `tb board`, e checagens baseadas no card como `max_concurrency`.
- L2, a escada de escalonamento por tier e a revisão entre vendors (ADR-013 §5).
- Agentes em outro host, e escolher um agente por capacidade ou tier.
- Impor o mandato nos caminhos de arquivo.

## Critérios de aceite

### B4a

1. **Revoke numa thread aberta.** O `Revoke` devolve `revoked`. Depois disso um `ClaimThread` falha com `thread revoked`, o `Complete` falha com `lease_lost` e o `Heartbeat` do dono lista a thread em `lost`, com o motivo.
2. **Revoke numa thread concluída.** Não muda nada e devolve `completed` com o `result_id`. Revoke numa thread sem claim bloqueia claims posteriores.
3. **`Revoke` e `Complete` concorrentes:** exatamente um vence, e o retorno do `Revoke` diz qual.
4. **Migração.** Um store v2 migra para v3 e mantém os dados; um store v4 é recusado.
5. **Marker.** O `ClaimConfined` grava o marker. O `tb bus claim` e o `tb_bus_claim` não conseguem defini-lo.
6. **Caminho feliz, com um router falso e um filho falso confinado.** O harness faz o claim, o filho edita o workspace e escreve um veredito `PASS`, e o harness conclui com esse veredito e o epoch certo.
7. **Um worker que perde o claim** nunca sobe um filho e registra `router_task_lost`.
8. **Permissões, num host com Landlock.** O filho consegue:
   - escrever no workspace;
   - ler `<root>/.git` (o `git status` funciona) e a cópia do ticket.

   Ele não consegue:
   - escrever em `<root>`, `<root>/.git/config`, `<root>/.toolbelt/bus/bus.db` nem no arquivo vivo do ticket;
   - abrir o `bus.db` de jeito nenhum.
9. **Entradas de `sandbox` recusadas.** Uma entrada de `sandbox.write` igual a `<root>`, `~` (quando a raiz está dentro dele) ou `<root>/.toolbelt` é recusada no `--check` e na hora do wake.
10. **Revoke durante o filho.** Um revoke mata o grupo de processos do filho em cerca de 2 s e registra `router_task_revoked`; nenhum complete vem depois.
11. **Vereditos ruins.** Um arquivo de veredito ausente, grande demais ou inválido, uma queda do filho e um timeout concluem cada um como `FAIL`, com texto fixo do harness.
12. **Não são tarefas do router.** Cada um destes é tratado como mensagem comum, sem claim:
    - um `router-task` de um agente com papel router que não é o fixado;
    - um de um supervisor;
    - um com `ticket: ../x`;
    - um com workspace em symlink;
    - um cuja thread difere da thread da mensagem.
13. **Sem rewake, sem tratar duas vezes.** Uma tarefa tratada não gera rewake, e um reinício do harness não a trata duas vezes.
14. **Sessões novas.** O preset `claude` ganha uma sessão nova por tarefa (sem `--resume`) e nenhuma tool do barramento.

### B4b

15. **Caminho feliz.** Só barramento, com um worker do B4a:
    - o ticket termina `done` com um commit `tb: <ID> <title>` cujo único pai é a base;
    - `bus_agent` e `bus_epoch` estão preenchidos;
    - o `tb merge <ID>` o aplica sem mudança.
16. **Assign.** O payload segue o `router-task/0.1`, com a thread `<ID>/<rand>.<attempt>` e `ttl` = `claim_timeout`. Ele não leva descrição, mandato nem texto de aceite.
17. **Nova tentativa.** `FAIL` ou um resultado malformado gera nova tentativa numa thread nova com o worktree resetado; depois de `max_attempts` o ticket fica `failed`.
18. **Agente faz commit.** Um commit do agente, uma troca de branch ou um HEAD desanexado falha na checagem antes do commit e escala sem nova tentativa. Um resultado que cita outro SHA não tem efeito. Um PASS sem mudanças fica `done` com `HeadCommit == BaseCommit`.
19. **Claims inválidos.** Cada um é revogado na hora e a tarefa vai de novo numa thread nova:
    - um claim sem marker;
    - um claim de um reviewer;
    - com `bus.to: agent:a`, um claim de `b`;
    - um claim feito antes do assign.

    Sem claim válido até o `claim_timeout`, a tentativa falha como `unclaimed`.
20. **Lease perdido.** Em cada caso o router revoga a thread, a tentativa falha com `bus_lease_lost` e o complete posterior do dono recebe `lease_lost`:
    - um dono calado além do lease, sem nenhum humano rodar o watchdog;
    - uma tomada por outro worker.
21. **Timeout.** `worker_slow`, depois revoke e `worker_stuck`. Um `PASS` concluído antes do revoke conta, pelo retorno `completed` do `Revoke`.
22. **Resultados velhos.** Nenhum destes marca um ticket como `done`:
    - um resultado de uma execução anterior do router;
    - um de um board recriado com os mesmos IDs de ticket;
    - um numa thread revogada.
23. **Reinício.**
    - Um reinício revoga as threads abertas, aceita uma concluída e devolve o resto para a fila.
    - Com o canal expirado, a `bus_thread` é mantida e revogada num início posterior, antes de o ticket ser atribuído de novo.
24. **O desligamento e o timeout global** terminam sem travar enquanto os slots do barramento estão ocupados. Uma tentativa em `committing` termina o commit.
25. **Sem cursor.** Um `tb bus recv --as router` durante uma execução não muda nada do que o router vê.
26. **Slots mistos.** Slots locais e do barramento rodam lado a lado; os locais são preenchidos primeiro.
27. **Canal expirado no meio de uma tentativa.** Uma tentativa com resultado gravado é aceita, as outras voltam para `pending`, um alerta `bus_channel_expired` é gerado e os slots locais seguem trabalhando.
28. **Config confiável.** Um `router.bus` definido no `.toolbelt/config.yaml` do repositório é ignorado; só a config global ou o `-c` contam.
29. **`--check` e `preflight`.**
    - O `--check` recusa sessão ausente, papel que não é router, canal expirado e um segundo membro com papel router.
    - O `preflight` recusa `max_workers: 0` sem `bus.channel`, e o aceita sem `agent_cmd` e sem Landlock quando `bus.channel` está definido.
30. **Assunto do commit.** Um título com caracteres de controle gera um assunto de commit sem eles, tanto para workers locais quanto do barramento.
31. **Summary.** `summary` só aparece no log do router, cortado, limpo e redigido, nunca no YAML do ticket.
32. **Kill do watchdog.** `tb watchdog kill b1` revoga a tentativa e libera o slot; a vigia do harness para o filho.
33. **Dry run.** O `--dry-run` mostra o slot `wN`/`bN` de cada ticket pronto, e o `state.json` mostra a thread, o agente e a fase de um slot do barramento.
