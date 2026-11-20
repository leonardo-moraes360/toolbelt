[🇺🇸 English](../../specs/appsec-playbooks.md) | 🇧🇷 Português

# Spec: playbooks AppSec red/blue (`tb red` / `tb blue`)

Autor: toolbelt · Status: proposto (não implementado) · Data: 2026-11-20 · Decisão: [ADR-017](../adr/017-appsec-red-blue-labs.md) · Companheiro: [red/blue labs](red-blue-labs.md)

## Visão Geral

`tb red` e `tb blue` dão ao agente metodologia AppSec nativa e nossa: um corpus **red** (ofensivo — como uma classe de vulnerabilidade se manifesta e é revelada em código ou serviço em execução) e um corpus **blue** (defensivo — detecção, hardening, resposta a incidente). O conteúdo é escrito no repo na voz do `tb`, escopado a segurança de software, sem dependência em runtime de qualquer corpus externo. Playbooks são indexados no grafo SQLite + FTS5 existente e servidos — compactos na busca, com orçamento no load — pela CLI e MCP.

Crucialmente, um playbook não é prosa inerte: um playbook blue pode emitir **checks executáveis** nos mecanismos de verificação existentes do `tb`, então o hardening vira algo que o projeto continua aplicando.

Este spec define o formato de playbook, a taxonomia no escopo, o pareamento red↔blue, o índice, a superfície de comando e MCP, o contrato de orçamento e o gancho de verificação. Não adiciona motor de armazenamento novo nem tooling executável de ataque próprio (execução acontece só dentro de um lab — ver spec companheiro).

## Objetivos / Não-Objetivos

**Objetivos**
- Corpus nativo e nosso — reimplementação, não vendorização (ADR-014/015).
- Escopar estritamente a AppSec / secure-SDLC.
- Pareamento red↔blue por classe de vulnerabilidade.
- Passos blue que compilam para checks `tb verify` / `tb guard` / `tb stack`.
- Reusar o grafo (ADR-003/011/014) e o servidor MCP (ADR-002); honrar a disciplina de token da ADR-011.

**Não-Objetivos**
- Sem amplitude de pentest (wireless, AD, C2, engenharia social) — extensão futura, fora de escopo agora.
- Sem ingestão de `SKILL.md` externo.
- Sem execução de payload nesta superfície; isso é trabalho do `tb lab` sob isolamento.

## Taxonomia no Escopo (inicial)

| Área | Classes (red) → espelho (blue) |
|------|-------------------------------|
| Web/API | injection (SQL/NoSQL/comando), XSS, SSRF, SSTI, XXE, IDOR, open-redirect, request-smuggling, deserialização, file-upload, abuso GraphQL |
| Auth | falhas de JWT, mau uso de OAuth/OIDC, lacunas de sessão/authz |
| Crypto | primitivas fracas, mau uso (IV/nonce/modo), config TLS |
| Supply chain | dependency confusion, typosquat, lockfile/integridade, secrets de build/CI |
| Secrets | secrets hardcoded, vazamento em logs/histórico, manejo de env |
| Plataforma | hardening de container, IAM/exposição de cloud, lacunas CSPM |

Toda classe red tem espelho blue (detecção + hardening + IR). A lista é o alvo da Fase 1, não um conjunto congelado.

## Formato de Playbook

Markdown nativo com frontmatter YAML, escrito sob `playbooks/`:

```
playbooks/
  red/<classe>/PLAYBOOK.md
  blue/<classe>/PLAYBOOK.md
```

Frontmatter:

```yaml
name: red-insecure-deserialization
color: red                         # red | blue
area: web
class: insecure-deserialization
mirror: blue-insecure-deserialization
triggers: ["deserialization", "gadget chain", "ObjectInputStream", "pickle", "unmarshal"]
summary: "Revela deserialização insegura: fontes, sinks, descoberta de gadget, impacto."
```

Seções do corpo (cabeçalhos estáveis, a unidade de fatiamento por orçamento):

- Red: `## Where It Hides`, `## Detection` (como confirmar), `## Exploitation` (metodologia, não payloads armados), `## Impact`, `## Remediation` (aponta para o espelho blue).
- Blue: `## Signals`, `## Detection Rules` (nossa sintaxe de regra ou referências), `## Hardening` (cada passo compilável para um check — ver abaixo), `## Incident Response`, `## References`.

## Gancho de Verificação (o que torna um playbook executável)

Um passo blue de `## Hardening` pode declarar um check:

```yaml
checks:
  - id: deser-no-native-java
    kind: guard            # guard | verify
    description: "Sem ObjectInputStream cru em entrada não confiável"
    detector: "grep"       # reusa um detector existente de tb guard/verify
    args: ["-RnE", "new ObjectInputStream\\(", "--include=*.java"]
    severity: high
```

`tb blue apply <classe>` registra os checks do playbook na config `tb guard`/`verify` do projeto (ADR-008), então o hardening é aplicado em todo `tb stack`. É a diferença concreta das bibliotecas de inspiração: a defesa persiste como um portão, não um parágrafo.

## Esquema do Índice (sobre a ADR-003)

Construído por `tb <color> index` no grafo DB:

| Coluna | Origem |
|--------|--------|
| name, color, area, class | frontmatter |
| summary, triggers | frontmatter (FTS) |
| path | `PLAYBOOK.md` relativo |
| sections | cabeçalhos `##` + faixas de bytes (fatiar) |
| checks | contagem + ids (só blue) |
| digest | sha256 (checagem de deriva) |

`play_edge(from, to, kind="mirror")` deriva do campo `mirror:`, bidirecional na consulta. Ranking reusa a config BM25 do grafo.

## Superfície da CLI

`tb red` liga a cor `red`, `tb blue` liga `blue`:

```
tb <color> list [--area A] [--json]
tb <color> search QUERY [--area A] [--limit N] [--json]
tb <color> load NAME [--section "H" | --budget TOKENS] [--full] [--json]
tb <color> pair NAME                 # resolve o espelho na outra cor
tb <color> index                     # (re)constrói o índice
tb blue    apply CLASSE [--dry-run]  # registra os checks do playbook blue em tb guard/verify
tb red     explain CLASSE            # red + espelho blue lado a lado (ataque ⇄ defesa)
```

- **search / list** — compacto, uma linha por hit: `[red/web] red-sqli — <summary>`, corpos nunca impressos.
- **load** — corpo sob `secplay.load_budget` (padrão 2000 tokens); acima do orçamento retorna frontmatter + índice de seções + top seções que couberem + `truncated: true`; `--section` retorna um bloco; `--full` ignora.
- **pair / explain** — seguem a aresta de espelho; `explain` renderiza ataque e defesa juntos.
- **blue apply** — escreve checks na config do projeto; `--dry-run` pré-visualiza.

## Superfície MCP (ADR-002)

| Tool | Anotação | Retorna |
|------|----------|---------|
| `tb_red_search` / `tb_blue_search` | readOnly | hits compactos |
| `tb_red_load` / `tb_blue_load` | readOnly | corpo com orçamento / seção nomeada |
| `tb_sec_pair` | readOnly | o playbook espelho |
| `tb_blue_apply` | **não** readOnly (`destructiveHint`, idempotente) | checks registrados na config guard/verify |

Tools de leitura são `readOnlyHint: true, openWorldHint: false`. `tb_blue_apply` muta a config do projeto, então é anotado conforme e aparece em `TestAllToolsAnnotated`. Via MCP, search/load nunca reindexam: em `stale: true` o chamador roda o tool `index` correspondente.

## Contrato de Orçamento de Tokens

1. A saída de `search`/`list` é só-cabeçalho, ≤ 1 linha por resultado.
2. `load` nunca excede `orçamento` tokens, a menos que `--full`/`full:true`.
3. Playbooks acima do orçamento retornam frontmatter + índice de seções + as top seções que couberem (por relevância à query, senão ordem do documento) com `truncated: true`.

## Modelo de Confiança

- Corpos de playbook são conteúdo nosso, revisado na autoria; ainda assim, o motor trata um corpo carregado como texto a exibir, nunca comandos a rodar (as garantias de conteúdo não confiável do egm, ADR-008, se estendem aqui).
- Nenhum caminho `tb red`/`tb blue` executa nada nem abre conexão de rede. Qualquer metodologia de exploração é informativa; a execução real fica confinada ao `tb lab` sob isolamento.
- `tb red` imprime o aviso de uso autorizado único (registrado em `.toolbelt`); `tb blue` não tem portão.

## Critérios de Aceite

- A1 — Todo playbook red no escopo resolve um espelho blue e vice-versa (`tb red pair`, `tb blue pair`).
- A2 — `tb red search "deserialization"` retorna `red-insecure-deserialization` compacto; corpo não impresso.
- A3 — `tb red load <grande>` acima do orçamento retorna `truncated: true`, índice de seções, ≤ orçamento tokens; `--section`/`--full` se comportam conforme contrato.
- A4 — `tb blue apply insecure-deserialization` registra os checks do playbook; um `tb guard`/`tb stack` seguinte os roda; `--dry-run` não escreve nada.
- A5 — ≥ 80% dos passos de hardening blue no corpus carregam um check executável.
- A6 — Nenhum caminho de leitura CLI ou MCP executa comando, abre socket ou emite segredo do usuário — garantido por teste.
- A7 — Tools MCP de leitura anotados `readOnlyHint:true, openWorldHint:false`; `tb_blue_apply` anotado destrutivo; todos em `TestAllToolsAnnotated`.
- A8 — Espelho EN + `docs/pt-BR/` presente (`TestDocsTranslationMirrorComplete`).

## Questões em Aberto

1. **Sintaxe de regra para `## Detection Rules` do blue.** Escrever nosso próprio formato compacto de regra, ou referenciar Sigma/YARA por exemplo sem depender de seus toolchains?
2. **Detectores de check.** Quais tipos de detector `tb guard`/`verify` expor aos checks de playbook (classe-grep agora; classe-AST/semgrep depois)?
3. **Local do corpus.** `playbooks/` na raiz do repo vs sob `docs/`? É dado consumido pelo `tb`, não prosa, então provavelmente um dir adjacente a código, indexado no build.
4. **Estimador de token compartilhado.** Confirmar que o estimador existente é exportado para `internal/secplay`, ou extraí-lo.
