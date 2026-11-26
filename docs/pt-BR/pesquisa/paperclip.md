[🇺🇸 English](../../pesquisa/paperclip.md) | 🇧🇷 Português

# O que o tb pode aprender com o Paperclip

> **Data da revisão**: 2026-11-26
> **Referência**: [paperclipai/paperclip](https://github.com/paperclipai/paperclip) no commit `dadbc495` (licença MIT)
> **Alvo**: `tb bus` ([internal/bus/](../../../internal/bus/), [internal/busrun/](../../../internal/busrun/)), `tb egm` ([internal/egm/](../../../internal/egm/)), [ADR-008](../adr/008-verificacao-adversarial.md) e o sandbox ([ADR-004](../adr/004-sandbox-isolamento.md))

## O que é o Paperclip

O Paperclip é um servidor Node.js/TypeScript com Postgres e UI React que orquestra "empresas" de agentes de IA: organograma, tickets, orçamento em dólar, etapas de aprovação, rotinas e vários tenants por instalação. Os agentes (Claude Code, Codex, Cursor, Gemini CLI, …) são acordados por um heartbeat e trabalham em tarefas.

É um produto diferente do tb. O tb é um CLI Go local, sem daemon, e os dois não têm código que dê para portar. O valor está nas decisões de design que o Paperclip já documentou para problemas que o tb também tem: trabalho que para pelo motivo errado, estado que sobrevive a uma release, entrada hostil e testes lentos. A licença é MIT, então ideias e trechos podem ser adaptados com atribuição.

## Achados, por valor para o tb

### 1. Task watchdog com fingerprint de parada

**Fonte**: [`doc/TASK-WATCHDOG.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/TASK-WATCHDOG.md)

**O que faz.** Um watchdog é ligado a uma tarefa e vigia essa tarefa e suas descendentes. Ele só dispara quando a subárvore inteira parou (sem execução viva, sem wake na fila, sem retry agendado). Aí calcula um SHA-256 sobre as folhas paradas (ids, status, blockers, interações pendentes) mais a própria configuração do watchdog e compara com `lastReviewedFingerprint`. Se bater, o mesmo estado parado já foi revisado e ele não faz nada. Um hash novo acorda um agente revisor com este mandato fixo:

- tratar cada folha parada como uma **alegação** a conferir contra evidência (CI, testes, comentários, screenshots). "Não consegui" e "aguardando aprovação" não são aceitos por padrão;
- deixar em paz as folhas de fato concluídas, com uma nota curta do que foi conferido;
- caso contrário, reabrir, reatribuir ou criar um follow-up dentro da subárvore; se o blocker for real, dizer quem desbloqueia e qual a próxima ação.

Regras que o impedem de entrar em loop ou ganhar poder:

- as tarefas de revisão dele levam um marcador de origem (`originKind = 'task_watchdog'`) e ficam fora da varredura, então ele não dispara a si mesmo;
- ele não pode acordar a si mesmo, editar a própria configuração nem criar outro watchdog;
- instruções customizadas só estreitam o foco; a autoridade é aplicada no servidor, rota a rota;
- ele não pode aprovar decisões formais nem substituir um revisor tipado.

O Paperclip separa três mecanismos que usam a palavra "watchdog": o **task watchdog** (subárvore parada), o **silent active-run watchdog** (processo rodando sem output por N segundos) e o **liveness recovery** (trabalho em andamento sem caminho vivo).

**Lacuna no tb.** O `tb bus watchdog` ([bus-lease.md](../bus-lease.md), R5) cobre só liveness: libera claims cujo dono perdeu a lease. Nada revisa um `complete` declarado sem prova, e o discriminador do ADR-008 roda por tarefa quando chamado, não quando uma árvore de trabalho chega ao repouso. Não foi achado em [internal/busrun/](../../../internal/busrun/) um watchdog de silêncio para o processo que o `tb bus run` executa.

**Proposta.** Uma "revisão de repouso" no bus: quando toda thread de um canal (ou de uma thread pai) estiver concluída ou liberada, calcular o hash do estado parado e, se for novo, publicar para o router uma tarefa de revisão com o mandato do ADR-008. Reusar o que já existe: o sender `system:*` para confiança, o fencing por epoch para o claim do revisor e um marcador de origem que tira as threads de revisão do próximo hash. Não precisa de daemon: a checagem pode rodar no `complete` que põe o canal em repouso, de acordo com o princípio 1 do bus-lease.md.

**Prioridade**: alta · **Módulo**: bus · router · verify

### 2. Estado de execução seguro entre releases

**Fonte**: [`rules/upgrade-safe-run-state.mdc`](https://github.com/paperclipai/paperclip/blob/dadbc495/rules/upgrade-safe-run-state.mdc)

**O que diz.** Estado salvo por uma release (sessões, leases, config de lançamento, checkpoints) é retomado pela seguinte com frequência, porque deploys acontecem no meio de execuções. Por isso:

- todo PR que muda estado persistido diz o que acontece com o estado salvo pela release anterior: segue igual, é migrado ou é recusado;
- a recusa usa um código de erro **permanente e nomeado**, nunca um genérico que dispara retry. Um retry não muda a identidade salva, e uma substituição pode destruir o estado salvo;
- a recusa mantém o estado salvo intacto;
- é melhor uma regra de admissão que classifique estado incompatível com a release do que mais um código de erro por caso;
- cada mudança ganha um teste de regressão salvo-no-formato-antigo, retomado-no-novo, mais um caso de deploy no meio da execução quando uma execução pode estar em andamento.

**Lacuna no tb.** Uma busca em `internal/` por `schema_version`, "cutover" ou "previous release" não acha nada. O store do bus, o índice do vault e os receipts do egm são todos estado persistido que um binário 1.0.x vai ler depois de um binário rc ter escrito.

**Proposta.** Antes da v1.0.0, listar todo formato persistido (schema SQLite do bus, índice em `.tb/`, receipts do egm, arquivos de lock e heartbeat do harness), dar uma versão a cada um e adicionar um teste salvo-antigo/lido-novo por formato, com um código de recusa nomeado. Adotar no `CONTRIBUTING.md` a pergunta de PR ("o que acontece com o estado salvo pela release anterior?").

**Prioridade**: alta (oportuna para o soak do 1.0.0-rc) · **Módulo**: bus · graph · egm · vault

### 3. Guarda de prompt injection em camadas, com fallback fail-closed

**Fonte**: [`doc/prompt-injection-prevention.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/prompt-injection-prevention.md)

**O que faz.** A detecção por regex embutida fica sempre ligada. Um classificador por modelo, opcional, recebe o resultado da tool com segredos redigidos e uma política fixa e versionada, e um veredito válido dele **substitui** o da regex, o que deixa passar exemplos benignos de segurança. Qualquer falha volta para a regex sobre o resultado original completo: sem budget, erro do provider, veredito inválido, timeout de 10 segundos ou corpo de requisição acima de 256 KiB (nunca truncado num prefixo). Se a redação esconder conteúdo que ainda seria entregue ao agente, o modelo é pulado (`input_redaction_mismatch`), para que instruções não se escondam em campos com nome de segredo. O log de auditoria registra qual detector decidiu e por que caiu no fallback. O conteúdo de `resources/read` do MCP passa pela mesma guarda que os resultados de tool.

**Lacuna no tb.** [internal/egm/injection.go](../../../internal/egm/injection.go) é só regex (`injection_patterns.txt`).

**Proposta.** Manter a regex como piso e adicionar um classificador opcional atrás do budget LLM do egm, com as mesmas regras: a regex sempre roda quando o modelo não pode, a entrada nunca é truncada, divergência de redação pula o modelo e o detector que decidiu fica registrado no finding.

**Prioridade**: média · **Módulo**: egm · guard

### 4. Preset de contenção de baixa confiança

**Fonte**: [`doc/LOW-TRUST-PRESETS.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/LOW-TRUST-PRESETS.md)

**O que faz.** O `low_trust_review` é para trabalho que lê entrada hostil (PRs não confiáveis, tickets externos, diffs de dependência). A política vem de três fontes (agente, projeto, execução), que são intersectadas, e a mais restritiva vence. Uma permissão arriscada sem escopo concreto falha fechada. Saída bruta não confiável nunca sobe automaticamente para o contexto de um agente de confiança maior, e um agente de baixa confiança não pode mudar instruções persistentes nem a configuração de skills.

**Relevância para o tb.** O sandbox e o modelo de capabilities já contêm o processo. O que falta ao tb é a regra de **contexto**: a saída de um worker que leu entrada não confiável não deveria ir para o prompt do planner ou do supervisor sem uma marca ou uma etapa de revisão. Isso cabe no router ([ADR-009](../adr/009-router-e-workers.md)) e nos labs red/blue ([ADR-017](../adr/017-appsec-red-blue-labs.md)).

**Prioridade**: média · **Módulo**: router · sandbox · lab

### 5. Budget de testes aplicado por script

**Fonte**: [`rules/test-budget.mdc`](https://github.com/paperclipai/paperclip/blob/dadbc495/rules/test-budget.mdc)

**O que diz.** Testes de PR não dormem mais de 1 segundo no relógio real, a não ser que a linha tenha uma anotação com o motivo. Suítes de soak, expiração de lease e restart ficam numa lista só-nightly. Suítes que cobrem invariantes centrais (isolamento, authz, atomic checkout, redação) nunca podem ir para o nightly. Um script (`scripts/check-test-budget.mjs`) aplica tudo isso nos arquivos alterados.

**Relevância para o tb.** Baixa por ora: as esperas em relógio real nos testes de `internal/bus` e `internal/busrun` ficam todas abaixo de 1 segundo (a maior é de 300 ms). A regra de "invariantes centrais nunca vão para o nightly" ainda vale ser escrita para os testes de lease e de fencing por epoch, se um nível nightly for criado.

**Prioridade**: baixa · **Módulo**: CI

## Valor baixo

- [`doc/memory-landscape.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/memory-landscape.md): um levantamento de sistemas de memória (mem0, MemOS, memU, supermemory, …), útil como insumo para [fontes-primarias.md](fontes-primarias.md).
- [`skills/para-memory-files`](https://github.com/paperclipai/paperclip/tree/dadbc495/skills/para-memory-files): memória de entidades em dois níveis (`summary.md` carregado primeiro, `items.yaml` sob demanda), próxima do que o `tb mem` já faz com resumos e fatos.
- [`doc/task-recovery.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/task-recovery.md): retry com `idempotencyKey` e `observedRunId`, mais regras para reusar o worktree de uma execução que falhou. O bus já tem operações idempotentes; a dica `priorWorkspace` (caminho, branch, lista limitada de arquivos não commitados) pode inspirar os retries do `tb bus run`.
- [`doc/UNTRUSTED-PR-REVIEW.md`](https://github.com/paperclipai/paperclip/blob/dadbc495/doc/UNTRUSTED-PR-REVIEW.md): revisão de PRs não confiáveis em Docker. O bubblewrap+Landlock já cobre isso.
- [`rules/*.mdc`](https://github.com/paperclipai/paperclip/tree/dadbc495/rules): regras de agente com escopo por glob e critérios de revisão, uma referência de formato para o `AGENTS.md`.

## Sem valor

Organograma, orçamento em dólar, várias empresas por instalação, a UI, plugins e conectores de chat servem a um plano de controle hospedado, não a um CLI local. O `tools/agent-shim` é um wrapper Go de 40 linhas que só faz `exec`.

## Ordem sugerida

1. Item 2 antes da v1.0.0, porque todo formato persistido sai junto com a release.
2. Item 1 como uma etapa do bus depois da release.
3. Itens 3 e 4 junto com o próximo trabalho no egm e no router.
