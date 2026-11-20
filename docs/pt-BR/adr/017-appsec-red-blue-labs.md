[🇺🇸 English](../../adr/017-appsec-red-blue-labs.md) | 🇧🇷 Português

# ADR-017: Capacidades nativas de AppSec red/blue e labs isolados (`tb red` / `tb blue` / `tb lab`)

## Status

proposto

## Contexto

`tb` é uma toolbelt para agentes de código, voltada a manter o software correto e barato de trabalhar. Uma extensão natural é mantê-lo **seguro**: dar ao agente conhecimento ofensivo (achar as classes de vulnerabilidade que entram no código) e defensivo (detectar, endurecer, responder), mais um lugar seguro para exercitar os dois.

Duas bibliotecas curadas de Claude Skills existem como inspiração:

- **[SnailSploit/Claude-Red](https://github.com/SnailSploit/claude-red)** — skills ofensivas (`SKILL.md`, primers de prompt).
- **[riparino/Claude-Cyber](https://github.com/riparino/Claude-Cyber)** — uma biblioteca defensiva `Claude-Blue` (detecção em Sigma/KQL/YARA, mapeamento MITRE) mais uma biblioteca red menor.

São amplas (wireless, Active Directory, C2, engenharia social) e são *material de prompt, não código*. Puxá-las inteiras tem três problemas: é uma dependência de conteúdo externo que deriva e precisa de licença e confiança; a maior parte da amplitude é irrelevante para segurança de **software**; e não acrescenta nada que o `tb` possa verificar ou rodar — é texto inerte.

Este projeto já estabeleceu o precedente para exatamente esta situação:

- **ADR-014** — *Graphify: reimplementar, não integrar.*
- **ADR-015** — *reimplementação nativa do Elephant-Goldfish Model.*

Aplicamos a mesma regra aqui. Também queremos que o agente possa **praticar e validar** trabalho de segurança sem pôr o host em risco — algo para o qual o `tb` está unicamente posicionado, já que possui um sandbox fail-closed (ADR-004).

Duas decisões de escopo foram tomadas ao escrever esta ADR:

1. **Escopo = AppSec / secure-SDLC.** No escopo: classes de vulnerabilidade de código (web, API, auth, crypto, injection, deserialização), dependências/supply-chain, secrets e hardening de container/cloud — com o par blue de detecção/hardening/resposta a incidente. Fora: amplitude de pentest (wireless, AD, C2, engenharia social/física). Registrado como extensão futura, não construído agora.
2. **Isolamento de lab = tiered.** Reusar o sandbox existente para passo único; usar **Podman rootless** (daemonless, sem root) para topologia de lab multi-serviço. Opcional e fail-closed.

## Decisão

Construir três capacidades nativas. Nenhuma depende de um corpus de segurança externo em build ou runtime; as bibliotecas-fonte são creditadas apenas como inspiração.

### 1. Playbooks AppSec nativos (reimplementados, não vendorizados)

Escrever nosso próprio corpus curado na voz do `tb`, escopado a segurança de software: um conjunto **red** (ofensivo — como uma classe de vulnerabilidade se manifesta e é revelada em código ou serviço em execução) e um conjunto **blue** (defensivo — assinaturas de detecção, passos de hardening, resposta a incidente). Cada playbook red é pareado ao seu par blue pela classe de vulnerabilidade (espelho). O conteúdo é nosso — escrito, revisado e mantido no repo — então pode ser mantido atual, verificado e licenciado sob a Apache-2.0 do repo.

### 2. Expostos como `tb red` / `tb blue`, indexados e com orçamento

Ambos os comandos são fronts finos sobre um motor `internal/secplay` ligado a uma cor. Os playbooks são indexados no grafo SQLite + FTS5 existente (ADR-003/011/014); `search` retorna linhas compactas, `load` retorna o corpo **sob orçamento de tokens** com fatiamento por seção, servido pelo servidor MCP (ADR-002) somente leitura. Mantém a disciplina de token da ADR-011: cabeçalhos na busca, detalhe sob demanda.

```
tb red  search "deserialization"
tb red  load red-insecure-deserialization [--section "Detection" | --budget 1500]
tb blue load blue-insecure-deserialization
tb red  pair red-sqli            # pula para o espelho blue, e vice-versa
```

### 3. Playbooks plugam na verificação do `tb`, não são inertes

Diferente da inspiração (texto de prompt puro), um playbook blue pode carregar **checks executáveis** nos mecanismos existentes do `tb`: um passo de hardening emite um check `tb verify`/`tb guard`; um passo de secure-SDLC pluga no fluxo `tb stack` / review (ADR-008). Assim, "endurecer contra X" vira um check que o projeto continua aplicando, não um parágrafo que o agente lê uma vez.

### 4. `tb lab` — labs red+blue isolados

Um novo motor `internal/lab` provisiona ambientes efêmeros e descartáveis para exercitar um ataque e sua defesa contra um **alvo local do lab**, nunca o host ou um terceiro. O isolamento é tiered:

- **Passo único → sandbox existente** (bubblewrap + Landlock, ADR-004): roda um payload/check fail-closed, sem dependência nova.
- **Topologia multi-serviço → Podman rootless**: um nó alvo (deliberadamente vulnerável ou o app do usuário), um nó red (tooling/agente), um nó blue (monitoramento/detecção), numa rede privada. Daemonless, sem root, sem grupo `docker`.

Labs são **isolados de rede do host por padrão** (sem egress), com teto de recursos, e **desmontados com garantia**. `tb lab` é **opcional e fail-closed**: sem Podman, labs de topologia recusam com mensagem clara e labs de passo único em sandbox ainda funcionam.

```
tb lab new sqli --target ./examples/vuln-api   # scaffold de definição de lab
tb lab up sqli                                  # provisiona (sandbox ou podman)
tb lab run sqli --red red-sqli --blue blue-sqli # exercita ataque + detecção
tb lab down sqli                                # teardown garantido
```

### 5. Confiança e uso duplo

Capacidade ofensiva é de uso duplo. `tb red` e `tb lab` exigem um reconhecimento de uso autorizado uma vez por repo. Corpos de playbook e imagens de lab/alvo são **conteúdo não confiável** sob o modelo egm (ADR-008): nada deles executa fora da fronteira sandbox/lab, e nenhum segredo do usuário sai do host. Labs por padrão não têm egress de rede; alcançar algo fora do lab é opt-in explícito e logado.

As superfícies completas vivem em [spec: AppSec playbooks](../specs/appsec-playbooks.md) e [spec: red/blue labs](../specs/red-blue-labs.md).

## Consequências

### Positivas

- **Sem dependência de conteúdo externo**: capacidade nativa, nossa, Apache-2.0, atual — coerente com ADR-014/015.
- **Escopada à missão**: AppSec/secure-SDLC serve o propósito de agente de código do `tb`; sem amplitude de pentest irrelevante.
- **Verificável, não inerte**: playbooks blue emitem checks reais `tb verify`/`guard`/`stack`.
- **Prática segura**: labs deixam o agente provar um ataque e sua detecção sem tocar o host, reusando o sandbox e adicionando só um runtime rootless opcional.
- **Honesto com tokens**: cabeçalhos na busca, load com orçamento, reusando o grafo.

### Negativas

- **Custo de autoria**: escrevemos e mantemos o corpus em vez de clonar — a troca deliberada da ADR-014/015.
- **Runtime opcional**: labs de topologia precisam de Podman; a ausência degrada o recurso (fail-closed) e adiciona um caminho de detecção de capacidade.
- **Superfície de segurança dos labs**: provisionar alvos vulneráveis e tooling ofensivo exige isolamento estrito, padrão sem egress e teardown garantido — um fardo de segurança real sobre o próprio `tb`.
- **Pressão de escopo**: usuários vão pedir a amplitude fora de escopo; precisamos segurar a linha ou faseá-la.

### Métricas de Sucesso

| Métrica | Alvo | Justificativa |
|--------|------|---------------|
| playbook_pairs_mirrored_pct | 100% | Todo playbook red no escopo tem espelho blue e vice-versa |
| blue_checks_runnable_pct | ≥ 80% | A maioria dos passos de hardening blue emite um check real `tb verify`/`guard` |
| lab_teardown_guaranteed | garantido por teste | Nenhum lab deixa containers, redes ou arquivos após `down`/falha |
| lab_default_no_egress | garantido por teste | Um lab novo não alcança o host nem a internet sem opt-in explícito |
| skill_load_within_budget_pct | 100% | Nenhum `load` excede o teto de tokens configurado |

## Alternativas Consideradas

### Vendorizar os repos red/blue como packs (rascunho anterior desta ADR)

- **Descrição**: copiar as bibliotecas `SKILL.md`, fixar um commit, servi-las.
- **Por que não**: dependência de conteúdo externo, deriva, fardo de licença/confiança, ~96k linhas de amplitude majoritariamente fora de escopo, e texto inerte que o `tb` não verifica. Rejeitado em favor de reimplementação (precedente ADR-014).

### Só docs, sem labs

- **Descrição**: distribuir playbooks, sem ambiente de execução.
- **Por que não**: perde o valor de validar com segurança; o agente não prova um ataque nem sua detecção, que é o ponto de parear red e blue.

### Docker para labs

- **Descrição**: usar Docker em vez de Podman rootless.
- **Por que não**: precisa de daemon e tipicamente do grupo `docker` (~root) — superfície de confiança maior que contradiz a postura fail-closed e de menor privilégio do `tb`. Podman rootless dá a mesma topologia sem daemon e sem root.

### Labs só em sandbox (sem runtime de container)

- **Descrição**: construir labs só sobre bubblewrap+Landlock.
- **Por que não**: bom para passos únicos, mas topologia de rede multi-nó (alvo/red/blue) é impraticável só com bubblewrap. Mantido como o tier de passo único, não a história toda.

## Referências

- [ADR-002](002-mcp-a2a-protocolos.md) — superfície do servidor MCP
- [ADR-003](003-sqlite-fts5-grafo.md) — grafo/índice SQLite + FTS5
- [ADR-004](004-sandbox-isolamento.md) — sandbox (bubblewrap + Landlock), fail-closed
- [ADR-008](008-verificacao-adversarial.md) — modelo de confiança adversarial / conteúdo não confiável
- [ADR-011](011-grafo-sob-demanda.md) — grafo sob demanda; skills humanas cortam até 41,73% do custo
- [ADR-014](014-graphify-reimplementacao.md) — reimplementar, não integrar
- [ADR-015](015-egm-nativo.md) — precedente de reimplementação nativa
- Inspiração: [SnailSploit/Claude-Red](https://github.com/SnailSploit/claude-red) · [riparino/Claude-Cyber](https://github.com/riparino/Claude-Cyber)
