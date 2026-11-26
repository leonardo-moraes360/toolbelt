[🇺🇸 English](../../pesquisa/alethe-cma.md) | 🇧🇷 Português

# O que o tb pode aprender com o alethe e o "plan big, execute small"

> **Data da revisão**: 2026-11-24 (registrada em 2026-11-26, quando as afirmações sobre o tb abaixo foram conferidas de novo na `main`)
> **Referências**:
> - [Kc1t/alethe-orchestrate-skill](https://github.com/Kc1t/alethe-orchestrate-skill) (HEAD `ab811045` em 2026-11-26)
> - [Kc1t/alethe-agents](https://github.com/Kc1t/alethe-agents) (HEAD `0acefe82` em 2026-11-26), principalmente `docs/ORCHESTRATOR_ROADMAP.md`
> - [anthropics/claude-cookbooks, `managed_agents/CMA_plan_big_execute_small.ipynb`](https://github.com/anthropics/claude-cookbooks/blob/main/managed_agents/CMA_plan_big_execute_small.ipynb) (HEAD `8ad5de64` em 2026-11-26)
>
> **Alvo**: `tb router` ([ADR-009](../adr/009-router-e-workers.md)), o bus ([ADR-013](../adr/013-barramento-multi-vendor.md) §5, [bus-router.md](../bus-router.md)), `tb bus run` ([internal/busrun/](../../../internal/busrun/)) e `tb bench` ([internal/bench/](../../../internal/bench/))

Os três tratam do mesmo problema que o router e o bus do tb: um modelo forte planeja e workers mais baratos executam. As partes mais úteis são coisas que o tb já planejou e ainda não fez. O [bus-router.md](../bus-router.md) lista "the escalation ladder by tier" e "picking an agent by capability or tier" como trabalho futuro.

## alethe-orchestrate-skill (maior valor)

1. **Tier mais profile.** As fases dizem só o tier (`cheap`, `standard` ou `strong`). Um profile (`base`, `savings`, `aggressive`) traduz cada tier em CLI, modelo e nível de esforço. Isso dá ao router e ao bus o roteamento por tier que falta, sem amarrar a um vendor.
2. **Playbooks declarativos em YAML.** Cada fase tem `role`, `tier`, `readOnly`, `workers`, `gate` e condições `after`/`if`/`then` (`conflict`, `checks-failed`). O tb tem tickets e board, mas nenhuma camada de "receita" por cima. Os playbooks que encaixam no tb:
   - `cascade`: o escalation ladder L2 do ADR-013;
   - `review-gate`: autor ≠ verificador, como no [ADR-008](../adr/008-verificacao-adversarial.md);
   - `scout-then-build`, `best-of-n` e `parallel-worktrees`;
   - `direct`: pula a orquestração em tarefa pequena.
3. **Uma pasta padrão por worker**: `prompt.txt`, `result.md`, `diff.patch` e `outcome.json` (status, tipo de falha, métricas, tripwire). O lead lê só o `result.md`, e a transcrição fica em disco. O report mede isso (por exemplo "5.0 KB on disk, 595 B read"), o que vira um KPI direto para o objetivo "economizar tokens" do tb.
4. **Gasto real, marcado `used`, `partial` ou `discarded`.** Tokens e custo vêm do que cada CLI reportou, separados entre trabalho aproveitado e descartado. O `internal/bench` já tem `Tokens` e `CostUSD`; os tickets do router ainda não levam isso.
5. **Quota guard.** Antes de cada onda ele lê o uso local de Claude e Codex (arquivos de sessão, sem rede) e se recusa a passar de uma linha de corte. Respeita a regra local-only do tb.
6. **Orçamento por onda e por orquestração.** O `tb bus run` limita só o número de wakes (`budget.max_wakes`, padrão 6). Não há limite em tokens nem em dólares.
7. **Journal de mudança de plano**: entradas `skip`, `add` e `switch`, cada uma com motivo, mostradas no report final.
8. **Endurecimento de worker.**
   - O ambiente do worker mantém só a auth do provider e tira o resto. O tb já faz isso: o `internal/execenv` filtra o ambiente do `verify`, do router e do `tb bus run`.
   - Tripwire: o git detecta um worker read-only que escreveu arquivos. É uma defesa extra quando um CLI de vendor roda fora do Landlock.
   - O git interno roda com hooks e fsmonitor desligados. O tb já faz isso no `egm`, no `workspace` e no `merge`.

## alethe-agents (app Tauri; pouco cabe)

O app em si (PTY, grid, temas, plugins) fica fora. A parte útil é o `docs/ORCHESTRATOR_ROADMAP.md`, que mapeou empiricamente o `claude -p --input-format stream-json --output-format stream-json`:

- **Processo vivo e multi-turn.** Um adapter persistente no `tb bus run` evitaria cold start a cada wake.
- **Um `rate_limit_event` a cada turno** (`rateLimitType`, `resetsAt`, `status`): sinal de quota de graça.
- **Um evento `result`** com `total_cost_usd`, `usage` com cache, `permission_denials` e `terminal_reason`: telemetria para o router sem trabalho extra.
- **Não há canal de aprovação.** O que precisa de permissão é negado na hora (`permission_denied`). Isso bate com a escolha do tb de dar allowlist fechada ao preset `claude`.
- **Interrupt** via `control_request` com `subtype: interrupt`. Não há como redirecionar no meio de um turno.

Ressalva: isso foi tirado do binário e não está documentado, então pode mudar entre versões.

Valor médio:

- **Ler e escrever a config MCP de cada vendor.** Uma etapa do `tb init` que registre o `tb mcp` em Claude, Codex, Cursor e OpenCode resolveria o onboarding que falhou no estudo de campo.
- **Handoff Claude→Codex por pacote redigido.** Encaixa no payload por referência do bus e na redação que ele já tem.

## Cookbook: CMA_plan_big_execute_small

- **Sustenta o padrão com números**: 84–98% do input da equipe foi cobrado na tarifa do worker.
- **Limite de segurança**: o orquestrador não tem tools e os workers recebem só as tools de que precisam. Isso pode virar um preset de mandato "reader", coerente com a regra do ADR-013 de que mensagem de outro agente é dado não confiável.
- **Orçamento aplicado em preço de lista cobrindo todas as threads.** Reforça o item 6 acima.
- **Método para o `tb bench compare`:**
  - o braço solo precisa cumprir a mesma exigência de verificação do braço com workers. Sem isso, o solo parece barato só porque faz um trabalho menos rigoroso;
  - delegar tem custo fixo por worker, e briefs mais finos encareceram as execuções, então a granularidade tem um ponto ótimo;
  - conferir a premissa da decomposição, não só os fatos. Na rodada deles, os dois braços montaram a mesma lista e erraram um item;
  - sem delegação, o tier forte foi pago à toa.
- **O orquestrador não vê o prompt dos workers.** No tb, o Agent Card enviado no `bus join` resolve isso, uma vantagem que o tb já tem.

## Ordem sugerida (depois da v1.0.0)

1. Tier e profile no router e no bus, junto com o `cascade` (o L2 planejado).
2. O formato do `outcome.json`, contabilidade de custo por ticket e orçamento em tokens ou dólares.
3. Um adapter `claude` stream-json persistente no `tb bus run`, usando `rate_limit_event` como quota guard.
4. Um catálogo de playbooks YAML por cima do router.
5. Um braço de "rigor equivalente" e um KPI de "fatia do input nos workers" no bench.

## Não vale trazer

Modo passivo via hook de plugin do Claude (o tb não é plugin), a UI desktop e o catálogo de plugins.
