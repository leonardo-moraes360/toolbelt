[🇺🇸 English](../../specs/red-blue-labs.md) | 🇧🇷 Português

# Spec: labs red/blue isolados (`tb lab`)

Autor: toolbelt · Status: proposto (não implementado) · Data: 2026-11-20 · Decisão: [ADR-017](../adr/017-appsec-red-blue-labs.md) · Companheiro: [AppSec playbooks](appsec-playbooks.md)

## Visão Geral

`tb lab` provisiona ambientes efêmeros e isolados para exercitar um ataque de segurança e sua defesa contra um **alvo local do lab** — nunca o host, nunca um terceiro. Deixa o agente provar que a técnica de um playbook red funciona e que o playbook blue pareado a detecta ou bloqueia, e depois joga o ambiente fora.

O isolamento é **tiered**:

- **Passo único → sandbox existente** (bubblewrap + Landlock, ADR-004): roda um payload ou check fail-closed, sem dependência nova.
- **Topologia multi-serviço → Podman rootless** (daemonless, sem root): um nó alvo, um nó red, um nó blue numa rede privada.

`tb lab` é **opcional e fail-closed**: se o tier necessário está indisponível, recusa claramente em vez de degradar o isolamento.

Este spec define a definição de lab, o ciclo de vida, o contrato de isolamento tiered, os padrões de segurança (sem egress, teto de recursos, teardown garantido), o modelo de ameaça e os critérios de aceite.

## Objetivos / Não-Objetivos

**Objetivos**
- Ambientes de exercício red+blue seguros e descartáveis dirigidos pelo `tb`.
- Reusar o sandbox (ADR-004) para o tier de passo único; adicionar só Podman rootless para topologia.
- Sem exposição do host: isolado de rede por padrão, com teto de recursos, desmontado em sucesso, falha e crash.
- Integrar com playbooks: `tb lab run` dirige um playbook red contra o alvo e um playbook blue como detector.

**Não-Objetivos**
- Não é um orquestrador geral de containers; só a topologia mínima alvo/red/blue.
- Nenhum ataque a qualquer coisa fora do lab; sem egress de internet por padrão.
- Nenhuma persistência de estado de lab além de uma exportação de artefato explícita e escopada.
- Nenhuma dependência de Docker ou daemon root.

## Modelo de Ameaça

- **Alvo e tooling hostis (coberto):** o alvo é deliberadamente vulnerável e o tooling red é ofensivo. Ambos não confiáveis. Não podem alcançar o filesystem do host, a rede do host, nem qualquer host externo. Egress é negado por padrão; os nós alvo/red/blue veem só a rede privada do lab.
- **Imagens de lab/alvo hostis (coberto):** imagens e definições de lab são conteúdo não confiável (ADR-008). O `tb` não roda instruções embutidas em imagem contra o host; só as provisiona dentro da fronteira de isolamento. A origem da imagem é fixada e registrada.
- **Tentativa de escape de um nó (mitigado, não eliminado):** Podman rootless + user namespaces + capabilities dropadas + seccomp + mounts do host read-only encolhem o raio de dano; um escape de container a nível de kernel permanece fora da garantia do `tb` e é documentado como risco residual.
- **Exaustão de recursos (coberto):** todo nó tem teto de CPU/RAM/PID/tempo (cgroups v2); um lab não pode matar o host de fome.
- **Estado residual (coberto):** teardown é garantido em todo caminho de saída; `tb lab verify` detecta órfãos.
- **Vazamento de segredo (coberto):** nenhum segredo ou env do host é injetado num lab; a allowlist de execenv (ADR) não se estende aos nós do lab.

## Definição de Lab

Escrita ou gerada por scaffold, guardada com o projeto (não em estado privado):

```
labs/<nome>/lab.yaml
```

```yaml
name: sqli
class: sql-injection            # liga ao par de playbook
tier: auto                      # auto | sandbox | podman
network:
  egress: deny                  # deny (padrão) | lista explicit-allow
  internal: true                # nós compartilham uma rede privada, sem rota pro host
nodes:
  target:
    image: <ref-fixada-ou-build>   # app vulnerável ou o ./app do usuário
    build: ./examples/vuln-api     # contexto de build local opcional
    expose: [8080]                 # só para a rede do lab
  red:
    image: <ref-de-tooling-fixada>
    playbook: red-sqli
  blue:
    image: <ref-de-monitor-fixada>
    playbook: blue-sqli
limits:
  cpus: "1.0"
  memory: "512m"
  pids: 256
  timeout: "10m"
```

`tier: auto` escolhe sandbox para um lab de nó único e passo único, e Podman para qualquer coisa com rede ou mais de um nó.

## Ciclo de Vida (CLI)

```
tb lab new NOME [--class C] [--target PATH]   # scaffold labs/NOME/lab.yaml
tb lab up NOME [--tier sandbox|podman]        # provisiona; fail-closed se tier ausente
tb lab status NOME                            # nós, rede, tetos, saúde
tb lab run NOME [--red P] [--blue P]          # dirige playbook red + detector blue; reporta
tb lab logs NOME [--node target|red|blue]
tb lab export NOME --to DIR                   # exportação de artefato escopada (opt-in)
tb lab down NOME                              # teardown garantido
tb lab list
tb lab verify                                 # detecta containers/redes/arquivos órfãos
```

O resultado de `tb lab run` é um veredito sobre o qual o agente pode agir: `attack=reproduced|failed`, `defense=detected|missed`, então um par red/blue pode ser validado ponta a ponta (e ligado ao `tb stack` depois).

## Contrato de Isolamento Tiered

| Preocupação | tier sandbox | tier podman |
|-------------|--------------|-------------|
| Motor | bubblewrap + Landlock (ADR-004) | Podman rootless, user namespaces |
| Topologia | nó único, sem rede inter-nó | alvo/red/blue numa rede interna |
| FS do host | Landlock read-only / negado | sem bind mounts do host exceto explícitos, read-only |
| Rede | nenhuma por padrão | só-interna; egress negado por padrão |
| Capabilities | seccomp fail-closed | drop-all + add mínimo, no-new-privileges |
| Teto de recursos | limites cgroup existentes | cgroups v2 por `limits` |
| Teardown | saída de processo | `podman ... --rm`, prune de rede + volume do label do lab |

**Fail-closed:** `tb lab up` sonda o tier primeiro. Podman ausente (ou sem suporte rootless, sem delegação cgroups v2) → recusa um lab podman com mensagem de remediação específica; nunca cai silenciosamente para isolamento mais fraco. Um lab tier sandbox sempre funciona.

## Padrões de Segurança

1. **Egress negado por padrão.** Um lab novo não alcança nem o host nem a internet. `network.egress` deve listar destinos permitidos explícitos para abrir qualquer coisa, e isso é logado.
2. **Sem segredos do host.** A allowlist de env/secret do host não propaga para os nós do lab.
3. **Teardown garantido.** `down`, saída de processo, timeout e crash todos convergem para limpeza completa; `tb lab verify` é a auditoria.
4. **Imagens fixadas.** Refs de imagem são fixadas (digest) ou construídas de um contexto no repo; registradas no manifesto do lab.
5. **Portão de uso autorizado.** Primeiro `tb lab` por repo imprime o aviso único (registrado em `.toolbelt`), compartilhado com `tb red`.

## Critérios de Aceite

- A1 — `tb lab up` num lab de topologia sem Podman recusa com mensagem de remediação e não provisiona nada (fail-closed); um lab tier sandbox ainda funciona.
- A2 — Um lab novo não tem rota pro host nem egress de internet; uma tentativa de egress de qualquer nó falha — garantido por teste.
- A3 — `tb lab down`, um timeout e um crash simulado cada um deixa zero containers, redes, volumes ou arquivos do label do lab; `tb lab verify` reporta limpo.
- A4 — `tb lab run sqli` reporta `attack=reproduced` e `defense=detected` para o lab SQLi semente contra seu alvo vulnerável.
- A5 — Nenhum segredo ou env do host alcança um nó do lab — garantido por teste.
- A6 — Todo nó tem teto por `limits` (cgroups v2); exceder um teto mata o nó, não o host.
- A7 — Podman é invocado rootless e daemonless; sem grupo `docker` ou daemon root.
- A8 — Espelho EN + `docs/pt-BR/` presente (`TestDocsTranslationMirrorComplete`).

## Questões em Aberto

1. **Detecção e delegação de Podman.** Como sondar Podman rootless + delegação cgroups v2 de forma portável (distros Linux variam; macOS precisa da VM da máquina Podman)? macOS provavelmente só tier sandbox na Fase 1.
2. **Imagens de alvo semente.** Distribuir um app vulnerável minúsculo e proposital por classe (nosso, mínimo), ou deixar o usuário apontar para o app dele? Provavelmente ambos: um alvo mínimo embutido para demos, `--target ./app` para uso real.
3. **Ligação do veredito de `tb lab run`.** Definir o contrato de veredito ataque/defesa e se vira uma op `tb stack` (como o egm fez).
4. **Origem das imagens.** Construir no repo a partir de Containerfiles (totalmente nosso, lento) vs fixar digests upstream (mais rápido, confiança externa)? Inclinando para no-repo para o alvo, fixado para camadas base.
5. **Rede rootless.** `pasta`/`slirp4netns` vs redes netavark só-internas para o padrão deny-egress; escolher o mais simples que garanta nenhuma rota pro host.
