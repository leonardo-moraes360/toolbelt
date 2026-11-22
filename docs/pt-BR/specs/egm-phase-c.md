[🇺🇸 English](../../specs/egm-phase-c.md) | 🇧🇷 Português

# `tb egm brainstorm`, `tb egm draft` e drift por símbolo — spec da Fase C

Autor: toolbelt · Status: **parcialmente implementada** (rodada 1, não revisada; D1–D7 resolvidas; fatias C1–C5 implementadas, C6 adiada para depois da 1.0) · Data: 2026-11-22 · Decisão: [ADR-015](../adr/015-egm-nativo.md) (§8) · Depende de: [núcleo](egm-nativo.md) (Fase A), [confiança](egm-trust.md) (Fase A.2), [drift e bootstrap](egm-drift-bootstrap.md) (Fase B)

## Visão geral

A Fase C fecha os três itens que o roadmap do README lista para o `tb egm`: os dois comandos de autoria do fluxo upstream ([ADR-007](../adr/007-inspiracao-elephant-goldfish.md): o elephant "faz brainstorm, escreve concept briefs e rascunha especificações") e o drift no nível dos símbolos de código que um documento cita, não só dos arquivos.

- **`tb egm brainstorm`** transforma um brief em um **concept brief**: ideias, trade-offs, uma direção recomendada e perguntas em aberto. É um artefato para o elephant e para o humano, **sem gate**, nunca revisado pelo tb e escrito fora do repositório por padrão.
- **`tb egm draft`** escreve a **primeira versão de uma spec ou ADR** a partir de um brief. O rascunho é só entrada para o loop de revisão existente: não ganha estado, recibo nem veredito, e só chega a `PASS` por `tb egm review` / `loop next`, como qualquer documento escrito à mão.
- **Drift por símbolo** permite que um documento cite `internal/x/x.go#Name`. A revisão confere que o símbolo existe, o recibo certifica os bytes do próprio símbolo e o `drift-report` informa `symbol_changed` / `symbol_missing`, tudo a partir de blobs, de forma determinística, sem LLM e sem banco do grafo.

A Fase C sai em slices (ver [Slices](#slices)); o C1 é um pull request.

## Estado atual

- **Comandos.** O `tb egm` tem `review`, `loop next`, `resolve`, `status`, `receipt` ([cmd/tb/egm.go:137](../../../cmd/tb/egm.go)), `bootstrap` ([cmd/tb/egm_bootstrap.go:21](../../../cmd/tb/egm_bootstrap.go)) e `drift-report` ([cmd/tb/egm_drift.go:23](../../../cmd/tb/egm_drift.go)). Nenhuma spec define `draft` ou `brainstorm`; a [egm-nativo.md](egm-nativo.md) (Não-objetivos) e a [egm-drift-bootstrap.md](egm-drift-bootstrap.md) (Fora da Fase B) os adiam para a Fase C.
- **Engines.** `auto`, `llm`, `deterministic` ([internal/egm/review.go:13-17](../../../internal/egm/review.go)); `auto` cai para `deterministic` sem `egm.agent_cmd` ([review.go:214](../../../internal/egm/review.go)).
- **Geração isolada já existe.** O goldfish de bootstrap do B3 ([internal/egm/bootstrapllm.go:28](../../../internal/egm/bootstrapllm.go)) roda um agente sem tools no `Runner` com sandbox ([internal/egm/run.go:114](../../../internal/egm/run.go), [run.go:227](../../../internal/egm/run.go)), limpa o corpo ([bootstrapllm.go:142](../../../internal/egm/bootstrapllm.go)), passa o scan de injeção e confere o conjunto de fontes R7. O esqueleto determinístico é [internal/egm/bootstrap.go:251](../../../internal/egm/bootstrap.go). O `draft` reaproveita os dois.
- **Personas.** Embutidas em `internal/egm/personas/`; `drift` e `bootstrap` ficam fora da lista de revisores. `inventive-catalyst` é uma persona revisora (procura alternativas melhores dentro de uma revisão), não um gerador de brainstorm.
- **Citações (R7).** Fontes são links Markdown (o fragmento é descartado) e spans entre crases terminados em extensão conhecida, sem `:` ([internal/egm/sources.go:45-66](../../../internal/egm/sources.go), [sources.go:127](../../../internal/egm/sources.go)). O checker `refs` confere se os caminhos citados existem ([internal/egm/checker_refs.go](../../../internal/egm/checker_refs.go)); nada confere um símbolo.
- **Recibos.** `Receipt` tem `claims`, `commit`, `doc`, `doc_sha256`, `sources`, `version` ([internal/egm/receipt.go:32-41](../../../internal/egm/receipt.go)); versões 1 e 2, a v2 só com `egm.receipt.write_v2` ([receipt.go:108](../../../internal/egm/receipt.go)). Os leitores usam `DisallowUnknownFields` ([internal/egm/ci.go:52](../../../internal/egm/ci.go)), então um campo novo exige uma versão nova. O CI confere o frescor só pelo hash do arquivo ([ci.go:110-119](../../../internal/egm/ci.go)). As rodadas do estado guardam `sources` como caminho → sha256 ([internal/egm/state.go:68](../../../internal/egm/state.go)).
- **O drift é por arquivo.** O `drift-report` compara os hashes de arquivo da base ([internal/egm/drift.go:308-355](../../../internal/egm/drift.go)); status `fresh`, `no_baseline`, `sources_differ`, `drifted`, `source_missing`, `unreadable` ([drift.go:45](../../../internal/egm/drift.go)). Qualquer edição num arquivo citado, mesmo longe do que o documento descreve, vira `drifted`.
- **O grafo não tem intervalos de linha.** `Node` não tem linha inicial/final ([internal/graph/graph.go:49-57](../../../internal/graph/graph.go)). Os IDs de função Go são `func:<pkg>.<name>` ([internal/graph/index_go.go:52](../../../internal/graph/index_go.go)): métodos perdem o receiver, então `(A) Name` e `(B) Name` no mesmo pacote dividem um ID; `funcIDs` é indexado pelo nome puro ([index_go.go:54](../../../internal/graph/index_go.go)) e `findFunc` devolve a primeira declaração com aquele nome ([index_go.go:76](../../../internal/graph/index_go.go), [index_go.go:106](../../../internal/graph/index_go.go)), então as arestas de chamada do segundo método vão para o primeiro. Os IDs de pacote são `pkg:<name>` ([index_go.go:34](../../../internal/graph/index_go.go)), então dois pacotes `main` dividem um nó. `SearchHit.Lines` só existe para chunks vetoriais ([internal/graph/search.go:38](../../../internal/graph/search.go)). O grafo é sob demanda e desligado por padrão ([ADR-011](../adr/011-grafo-sob-demanda.md)).

## Objetivos

1. Um elephant (ou um humano com `egm.agent_cmd`) consegue ir de brief → concept brief → spec em rascunho → loop de revisão com comandos do tb, e todo texto gerado continua sendo dado não confiável até uma revisão normal convergir.
2. Um rascunho nunca aprova a si mesmo: nada que o tb escreve para `draft` ou `brainstorm` conta para veredito, recibo ou gate de CI.
3. Um documento pode amarrar uma afirmação a um símbolo Go, e o drift desse documento informa o símbolo, não o arquivo inteiro.
4. Tudo o que envolve símbolos é Go puro e determinístico sobre blobs do git: mesmo commit, mesmo build do tb, mesma resposta; sem LLM, sem rede, sem depender do grafo estar ligado.

## Não-objetivos

- O tb editar um documento existente (o princípio 2 da Fase B continua): `draft --update` escreve uma proposta fora da árvore.
- Encadear `draft` em `review` automaticamente, ou abrir linhagens de revisão a partir da saída do brainstorm.
- Símbolos para linguagens além de Go (os indexadores de Python, Rust e JS podem seguir o mesmo contrato depois).
- Ferramentas MCP para os comandos novos (levam minutos; só CLI, como `review` e `bootstrap`).
- Os itens da Fase C do ADR-015 adiados em [Escopo](#escopo-dos-itens-da-fase-c-do-adr-015).

## Princípios

1. **O tb decide, o LLM escreve ou julga.** O tb coleta, redige, faz scan, monta, faz hash e registra. Uma chamada de modelo é um processo isolado, como um goldfish, sem tools (isolamento da Fase A).
2. **Texto gerado é dado.** É limpo, passa pelo scan de injeção e pela checagem de fontes antes de tocar o disco; leva uma linha de status de gerado; o tb nunca faz stage nem commit dele.
3. **Sem autoaprovação.** A geração não escreve estado de revisão, rodada nem recibo; a persona geradora é recusada como revisora e como árbitro.
4. **Offline por padrão.** Sem `egm.agent_cmd`, os caminhos com LLM recusam com `agent_unavailable`; os caminhos determinísticos (`draft --body`, drift por símbolo) nunca chamam um agente.
5. **Símbolos têm escopo de arquivo.** Um símbolo é identificado pelo caminho citado mais um nome dentro daquele arquivo, nunca por um ID global do grafo, então colisões de ID do grafo não chegam ao egm.

## Parte 1: `tb egm brainstorm`

### Comando

```bash
tb egm brainstorm --brief rd.md
tb egm brainstorm --brief rd.md --from internal/router/ --out notes/router-ideas.md
tb egm brainstorm --brief rd.md --json
```

**R1. Brief.** `--brief` é um arquivo dentro do repositório: regular, sem symlink em nenhum componente, fora de `.git`, com no máximo `max_source_bytes`. Pode não estar rastreado (um `rd.md` em geral não está). O tb o lê, aplica `Redact`, recusa um brief `secret_like` com `brief_secret` e roda `ScanInjectionSource`; um acerto recusa com `brief_injection` antes de qualquer agente rodar. O brief é um bloco DATA, nunca parte das instruções.

**R2. Contexto.** `--from` (opcional, repetível) segue exatamente as regras do bootstrap (só arquivos rastreados, regras de caminho da Fase A, limites do R7, `secret_like` pulado e listado). O brainstorm não liga `## Sources`: o concept brief não tem gate, então suas citações não são certificadas.

**R3. Engine.** Só LLM. Um goldfish com a persona embutida `brainstorm` (nova; texto original), o isolamento da Fase A e a verificação de uso de tools; uma nova tentativa, como no bootstrap. Sem `egm.agent_cmd`: exit 2, `agent_unavailable`. Com `tools_check` desligado: `tools_unverifiable`. A execução conta contra `egm.max_agent_runs`, `max_stdout_bytes` e `timeout_per_reviewer`.

**R4. Contrato de saída.** Um corpo Markdown de no máximo `egm.brainstorm.max_bytes` (padrão 16 KiB; chave de limite, a config do repositório só pode diminuir) com estas seções de nível 2, nesta ordem: `## Problem`, `## Ideas` (cada ideia um título de nível 3 com seus trade-offs), `## Recommended direction`, `## Open questions`. O tb aplica a limpeza do bootstrap (comentários HTML até não sobrar nenhum, títulos de nível 1, linhas `Status:` antes do primeiro `##`) e o scan de injeção no corpo (`brainstorm_injection`); uma seção faltando ou um corpo vazio, grande demais ou fora de UTF-8 duas vezes dá `brainstorm_invalid`.

**R5. Onde cai.** O tb acrescenta um título (do primeiro título do brief, senão do nome do arquivo) e a linha de status `Status: **concept brief, generated by tb egm brainstorm** (not gated, not reviewed).`

- **Padrão:** `<state>/egm/<repo-id>/briefs/<slug>-<n>.md` (a raiz de estado da Fase A, fora do repositório, modo `0600`, `n` monotônico). Nada muda na árvore, então não há gate, teste de espelho nem commit envolvido.
- **`--out <path>`:** um arquivo novo dentro do repositório, com as regras de alvo do bootstrap (`O_CREAT|O_EXCL`, diretório pai existe, sem symlink em nenhum componente, fora de `.git`, não negado pelo R7). Um caminho que casa com `egm.doc_globs` é recusado com `brainstorm_gated`: um concept brief não é uma spec, e colocá-lo onde o gate olha deixaria a própria saída do tb com gate por acidente.

`--json` imprime `{"brief_sha256", "engine", "out", "v": "egm-brainstorm/0.1", "warnings"}`. O tb imprime o próximo passo: `tb egm draft <doc> --brief <out>`.

## Parte 2: `tb egm draft`

### Comando

```bash
# Corpo escrito pelo elephant (determinístico: o tb só monta e confere)
tb egm draft docs/specs/router.md --brief rd.md --from internal/router/ --body /tmp/body.md

# Goldfish redator isolado
tb egm draft docs/specs/router.md --brief rd.md --from internal/router/ --engine llm

# Proposta para um documento existente com gate (slice C3)
tb egm draft docs/specs/router.md --update --brief rd.md --engine llm
```

**R6. Dois jeitos de escrever, uma montagem.** O elephant do ADR-007 tem o contexto inteiro; o tb não. Então o `draft` aceita os dois:

- **`--body <file>` (determinístico, o padrão quando dado).** O elephant escreve o corpo com o próprio contexto; o tb monta e confere. Nenhum agente roda. É o mapeamento fiel da skill `draft` do upstream.
- **`--engine llm`.** Um goldfish redator isolado (persona embutida `draft`, nova) recebe o padrão derivado, os títulos obrigatórios, o brief redigido e os arquivos `--from` aceitos como blocos DATA, e devolve o corpo. Mesmo isolamento, nova tentativa, orçamentos e recusas do R3 (`agent_unavailable`, `tools_unverifiable`).

Passar os dois, ou nenhum, sai com 2 e `usage`. Não há gerador determinístico de corpo: o `tb egm bootstrap` já escreve o esqueleto.

> **Resolvida, D1.** Manter os dois modos: `--body` (determinístico) e `--engine llm`. (Alternativas consideradas: só `--body`, ou só `--engine llm`.)

**R7. Alvo e tipo.** As regras de alvo do bootstrap: arquivo novo (`O_CREAT|O_EXCL`), casa com `egm.doc_globs`, tipo derivado de `egm.doc_types` (sem `--type`). As recusas reaproveitam os códigos do bootstrap renomeados para o draft: `draft_exists`, `draft_path`, `draft_sources`.

**R8. Brief.** Como no R1 (`brief_secret`, `brief_injection`). `--brief` também pode apontar para um concept brief em `<state>/egm/<repo-id>/briefs/` deste repositório. O brief **não é citado** no rascunho: um brief não rastreado seria um pulo do R7 e começaria a primeira revisão rebaixada (`ci_partial` no CI). O rascunho precisa se sustentar sozinho; os revisores veem só ele e suas fontes.

**R9. Montagem.** O tb monta o arquivo como o esqueleto do bootstrap (título, linha de status, seções obrigatórias, `## Sources` com links relativos para os arquivos `--from` aceitos), com o corpo no lugar das seções `TODO`, e:

- a linha de status é `Status: **draft, generated by tb egm draft** (not reviewed).`;
- o corpo passa pela limpeza do bootstrap e depois pelo `injection-scan` da Fase A (`draft_injection`);
- um corpo que omite uma seção obrigatória mantém essa seção com a linha `TODO` (o checker `required-sections` da revisão então a aponta);
- a coleta R7 do arquivo montado precisa dar exatamente os arquivos `--from` aceitos; qualquer outra citação, ou qualquer arquivo aceito que o R7 excluiria, recusa com `draft_sources`;
- um corpo vazio, grande demais (`egm.draft.max_bytes`, padrão 32 KiB, chave de limite) ou fora de UTF-8 recusa com `draft_invalid`.

Nada é escrito numa recusa.

**R10. Sem autoaprovação.**

- O `draft` não escreve estado, rodada, linhagem, convergência nem recibo, e não lê nenhum. A única exceção é o `draft --update`, que cria `<state>/egm/<repo-id>/<slug>/drafts/` para a proposta (sem `state.json`, rodada nem recibo).
- As personas `draft` e `brainstorm` são recusadas por `--reviewers` e nunca carregadas pelo árbitro (como `drift` e `bootstrap`).
- `tb egm receipt` num documento rascunhado falha exatamente como em qualquer documento não revisado (sem convergência); o CI exige a checagem completa da A.2 para um documento novo com gate de qualquer forma.
- A linha de status de gerado é um fato sobre o arquivo, não uma entrada do gate. **Resolvida, D2:** nenhum achado. Nenhum checker aponta uma linha de status de gerado que sobrou; os revisores e o autor a veem no documento.

**R11. `--update` (slice C3).** Para um documento existente com gate, o tb nunca o edita. O goldfish redator (ou `--body`) produz uma versão nova completa, montada e conferida como no R9 (a linha de status não muda; a `## Sources` existente é mantida e o novo conjunto ainda precisa ser exatamente o que o R7 resolve), e o tb a escreve em `<state>/egm/<repo-id>/<slug>/drafts/draft-<n>.md` e imprime um diff unificado (o `DiffNoIndex` endurecido do tb). O elephant aplica; o próximo `tb egm review` vê um novo `doc_sha256`, então a convergência anterior fica `STALE`, como em qualquer edição.

**R12. Saída.** O texto imprime o alvo, os arquivos aceitos e pulados e o próximo passo (`tb egm review <doc>`; neste repositório também o espelho pt-BR). `--json`: `{"doc", "engine", "skipped", "sources", "v": "egm-draft/0.1", "warnings"}` (`proposal` e `diff` no lugar de `doc` para `--update`). O tb nunca faz stage nem commit.

## Parte 3: drift por símbolo

### O que é um símbolo

**R13. Símbolos Go (extrator puro).** Uma função pura nova em `internal/graph`, `GoSymbols(src []byte) ([]Symbol, error)`, faz o parse dos bytes de um arquivo com `go/parser` (sem sistema de arquivos, `SkipObjectResolution`, comentários mantidos para as posições) e devolve as declarações de nível superior:

| Tipo | Nome na citação | Intervalo de bytes com hash |
|---|---|---|
| `func` | `Name` | da palavra `func` ao fim do corpo |
| `method` | `Recv.Name` (ponteiro e parâmetros de tipo removidos: `(*T[K]) M` → `T.M`) | idem |
| `type` | `Name` | o `TypeSpec` (o próprio intervalo de linhas; num `type (...)` agrupado, só o spec) |
| `const`, `var` | `Name` | o `ValueSpec` que o declara |

Cada `Symbol` tem `Name`, `Kind`, `StartLine`, `EndLine` (a partir de 1, inclusivo) e o `SHA256` dos **bytes crus** do intervalo. O comentário de documentação fica fora do intervalo (editar um comentário não é drift). `_`, `init` e qualquer nome declarado duas vezes no arquivo (um arquivo que não compila) são **ambíguos**. Um erro de parse deixa todos os símbolos daquele arquivo indisponíveis (`unparsed`). Arquivos acima de `max_source_bytes` não passam pelo parse.

> **RESOLVIDO D3 (usuário, 2026-11-22): hash dos bytes crus.** Fazer hash dos bytes crus (recomendado: estável entre builds do tb; um reflow só de `gofmt` conta como mudança) ou da saída do `go/printer` (ignora o layout, mas a saída pode mudar entre versões do Go, então dois builds do tb poderiam discordar).

### Citar um símbolo

**R14. Sintaxe.** Uma citação de um arquivo `.go` pode levar um fragmento de símbolo:

- link: `[Drift](../../internal/egm/drift.go#Drift)`, `[ExitCode](../../internal/egm/drift.go#DriftReport.ExitCode)`;
- span: `` `internal/egm/drift.go#Drift` ``.

O fragmento precisa casar com `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$` e não pode ser uma âncora de linha (`^L[0-9]+(-L[0-9]+)?$`, mantida como hoje: descartada, por arquivo). Qualquer outro fragmento é descartado como hoje. O arquivo continua sendo uma fonte R7 (regras de caminho, limites e pacote não mudam). A forma de span é nova para o R7: hoje um span terminado em `#Name` não é candidato ([sources.go:59](../../../internal/egm/sources.go)); depois do C4 o caminho dele passa a ser, então um documento que já tinha um span assim ganha uma fonte (`sources_differ` uma vez). Não existe span assim nos docs deste repositório; o CHANGELOG registra isso.

**R15. Fonte com escopo de símbolo.** Um caminho de fonte citado **só** por citações de símbolo tem *escopo de símbolo*; um caminho também citado sem fragmento (um link ou span sem fragmento de símbolo) tem *escopo de arquivo*, como hoje.

### Revisão

**R16. Verificação (checker `refs`).** Para cada citação de símbolo cujo arquivo resolve, o tb roda `GoSymbols` no conteúdo do snapshot: um nome ausente dá um achado `refs` em `major` ("symbol `X` not found in `path`"), um ambíguo dá `symbol_ambiguous` em `major`, um arquivo sem parse dá um achado `nit` por arquivo. Determinístico; roda também com `--engine deterministic`. É a "verificação de símbolos no grafo" do ADR-015, feita no snapshot em vez do índice do grafo, então funciona com o grafo desligado e não lê um índice desatualizado.

**R17. Estado da rodada.** Cada rodada registra `symbols` (`"path#Name"` → sha256) ao lado de `sources` ([state.go:68](../../../internal/egm/state.go)) para cada símbolo citado, resolvido e não ambíguo. Rodadas sem o campo são lidas como "sem base de símbolos".

**R18. Recibo versão 3.** A v3 é a v2 mais `symbols` (um mapa, possivelmente vazio, com o mesmo formato de chave; chaves ordenadas pela codificação determinística existente). Os leitores aceitam 1, 2 e 3; o tb escreve v3 só com a chave exclusiva do usuário `egm.receipt.write_v3: true` (que implica o `commit` da v2), então um CI fixado num tb mais antigo é atualizado antes, como no rollout do B3. Um recibo v3 cujas chaves de `symbols` não são exatamente as citações de símbolo resolvidas do documento, ou cujos valores não são hex de 64 caracteres, é `receipt_invalid`.

**R19. Frescor no CI.** O `fresh` ([ci.go:110](../../../internal/egm/ci.go)) também recalcula cada hash de símbolo da v3 no head; uma diferença é a mesma falha de uma diferença de hash de fonte. Edições de recibo que só reescrevem `symbols` selecionam o documento (a checagem 1 do B1 já cobre qualquer edição de recibo).

### Drift

**R20. Comparação.** Para uma base (recibo v3, ou uma rodada do estado com `symbols`):

- uma fonte com **escopo de arquivo** é comparada pelo hash do arquivo, exatamente como na Fase B;
- uma fonte com **escopo de símbolo** é comparada pelos seus símbolos: cada símbolo da base é extraído em `--at` e seu hash comparado. Um arquivo cujo hash mudou enquanto todos os símbolos citados ficaram iguais **não** é drift: o documento continua `fresh` (se nada mais mudou) e o caminho aparece em `outside_symbols` (informativo, exit 0);
- com um recibo v1/v2 ou uma rodada sem `symbols`, toda fonte tem escopo de arquivo (comportamento inalterado).

**R21. Status e campos novos.**

| Status | Significado | Exit |
|---|---|---|
| `symbol_changed` | o hash de um símbolo citado mudou (`symbols_changed`: `{"path", "symbol"}`) | 1 |
| `symbol_missing` | um símbolo citado não resolve mais (`symbols_missing`: `{"path", "symbol", "reason"}`, reason `deleted`, `ambiguous`, `unparsed`, `too_large`) | 1 |

Um `source_missing` por arquivo de um caminho com escopo de símbolo continua `source_missing`. Ordem de severidade: `unreadable` > `source_missing` > `symbol_missing` > `drifted` > `symbol_changed` > `sources_differ` > `fresh` > `no_baseline`. O JSON ganha `symbols_changed`, `symbols_missing`, `outside_symbols` (sempre presentes, possivelmente vazios) e sobe `v` para `egm-source-drift/0.2`.

> **RESOLVIDO D4 (usuário, 2026-11-22): uma mudança fora dos símbolos citados é informativa (`outside_symbols`), não drift.** O R20 deixa o drift *menos* conservador para fontes com escopo de símbolo: uma mudança num helper que o símbolo citado chama não é informada. Recomendado: aceitar (citar o arquivo sem fragmento devolve o drift por arquivo, e `outside_symbols` mantém a mudança visível). Alternativa: informar os status de símbolo só como detalhe extra e manter `drifted` para qualquer mudança no arquivo.

**R22. CI e gate local.** O `ci_source_drift` (B1) e o aviso local `sources_changed` usam o R20, então disparam com `symbol_changed` / `symbol_missing` e não com `outside_symbols`. O goldfish de `drift` (`--engine llm`) recebe, para fontes com escopo de símbolo, os intervalos dos símbolos alterados marcados no pacote; fora isso, não muda.

### Grafo (slice C6)

**R23. Intervalos de linha.** `Node` ganha `start_line` e `end_line` (`omitempty`, aditivos ao índice JSON). O indexador Go os preenche a partir de `GoSymbols`, então o grafo e o egm dividem um extrator. `tb graph explain` imprime `path:start-end` quando presentes.

**R24. IDs de método com receiver.** Métodos passam a `func:<pkg>.<Recv>.<Name>`; `funcIDs` e `findFunc` usam o nome qualificado, corrigindo a atribuição errada de arestas em [index_go.go:54-76](../../../internal/graph/index_go.go). Os IDs de pacote continuam `pkg:<name>` neste slice (IDs por import path são uma mudança maior).

> **RESOLVIDO D5 (usuário, 2026-11-22): a mudança dos IDs de método fica adiada para a C6, pós-1.0; C4 e C5 não tocam nos IDs do grafo.** O R24 muda IDs de nó existentes do grafo (consultas salvas, argumentos de `tb graph path`, testes golden). Entregar no C6 com uma entrada "changed" no CHANGELOG (recomendado), ou manter os IDs antigos e pôr o receiver só no `title`.

## Escopo dos itens da Fase C do ADR-015

| Item | Fase C (esta spec) | Por quê |
|---|---|---|
| `draft`, `brainstorm` | incluídos (C1–C3) | roadmap do README |
| verificação de símbolos no grafo | incluída (C4), feita sobre snapshots | exigida pelo drift por símbolo; o grafo continua sob demanda |
| drift por símbolo | incluído (C5) | roadmap do README |
| intervalos de linha e IDs de método no grafo | incluídos (C6) | pedidos pelo trabalho com símbolos; independente do egm |
| outros presets (`staged`, `feature`, `implementation`) | adiados | cada um precisa dos próprios padrões e dados de convergência |
| personas customizadas por caminho | adiadas | texto de persona vindo do repositório é um prompt que o repositório controla; precisa de um desenho de confiança próprio |
| perfil `codex` | adiado | precisa de um spike no estilo do R0 (tools desligadas, detecção de uso de tools, sem carregar config do usuário) |
| override de agente por revisor | adiado | depende de um segundo perfil |
| churn | adiado | ainda não definido em lugar nenhum |
| adaptador `review.Finding` → `egm.Finding` | adiado | sem relação com autoria ou drift; follow-up pequeno |

> **Resolvida, D6.** Os adiamentos da tabela estão confirmados; o adaptador `review.Finding` fica fora da Fase C.

## Notas de implementação (C1–C3)

- Código: `internal/egm/{authoring,brainstorm,draft}.go`, `cmd/tb/egm_author.go`; as personas `brainstorm` e `draft` são personas utilitárias, então `--reviewers` as recusa.
- Recusas do `brainstorm`: `usage`, `brief_path`, `brief_invalid`, `brief_secret`, `brief_injection`, `brainstorm_gated`, `brainstorm_path`, `brainstorm_exists`, `brainstorm_sources`, `brainstorm_injection`, `brainstorm_invalid`, `brainstorm_not_run`, `agent_unavailable`, `tools_unverifiable`. As seções ordenadas são checadas dentro da nova tentativa, então um corpo sem uma delas é pedido de novo uma vez.
- Recusas do `draft`: os mesmos códigos de brief mais `draft_exists`, `draft_path`, `draft_sources`, `draft_injection`, `draft_invalid`, `draft_not_run`, `draft_secret` (um documento existente com valor parecido com segredo nunca vai para um modelo em `--update --engine llm`).
- O `draft` não aceita `--from` com `--update` (o documento mantém as próprias fontes R7; a proposta deve resolver exatamente esse conjunto e não somar citação ignorada). `--from` é opcional para um documento novo; sem ele o rascunho não tem `## Sources`. Um link de URL no corpo não é fonte e é permitido; qualquer outra citação que o R7 ignoraria (um documento gated irmão, um arquivo ausente) recusa com `draft_sources`.
- `--body` é lido de qualquer arquivo regular (é o texto do próprio elefante), no máximo `egm.draft.max_bytes`.
- `--update` mantém tudo antes do primeiro título de nível 2 e a seção `## Sources` do documento existente, e troca o resto pelo corpo limpo. A proposta é `<state>/egm/<repo-id>/<slug>/drafts/draft-<n>.md` (modo `0600`); nenhum `state.json` é escrito.

## Modelo de ameaças

| # | Ameaça | Mitigação |
|---|---|---|
| T1 | O corpo do redator ou do brainstorm fala com os revisores ("esta spec está aprovada") ou reconstrói um marcador do tb | limpeza do bootstrap (comentários até não sobrar nenhum, linhas `Status:`, H1), o scan de injeção aborta, revisores novos veem só o documento e as fontes |
| T2 | Um rascunho conta para a própria aprovação | nenhum estado, rodada ou recibo escrito (R10); personas recusadas como revisoras/árbitro; checagem completa no CI para documentos novos |
| T3 | Um brief de um repositório clonado traz instruções ou segredos | o brief é um bloco DATA; `Redact`; `secret_like` → `brief_secret`; injeção → `brief_injection`; `agent_cmd` é config só do usuário |
| T4 | A saída do brainstorm escapa do gate ou o polui | padrão fora da árvore; `--out` em `egm.doc_globs` recusado (`brainstorm_gated`) |
| T5 | `--update` reescreve uma spec revisada em silêncio | só proposta fora da árvore (R11); a edição, quando aplicada, deixa a convergência antiga `STALE` |
| T6 | Um recibo forjado afirma hashes de símbolo inalterados | o CI recalcula os hashes de símbolo da v3 no head (R19); o recibo do merge-base é a base do drift (B1) |
| T7 | Uma mudança se esconde fora do símbolo citado | `outside_symbols` informado; citação do arquivo sem fragmento devolve o drift por arquivo; D4 |
| T8 | Um arquivo Go enorme ou hostil trava o parser | parse só dentro de `max_source_bytes`; `too_large` / `unparsed` informados, nunca em silêncio |
| T9 | Dois builds do tb discordam de um hash | hash dos bytes crus (D3); falha de parse informada como `unparsed`, não como hash diferente |
| T10 | Rede | o tb não abre nenhuma; só o processo filho do agente que o humano configurou fala com o provedor |
| T11 | Um pull request diminui limites ou troca o modo de símbolo para esconder drift | o escopo de símbolo vem do texto do documento e da versão do recibo, não da config; o CI usa as configurações do merge-base (B1) |

## Critérios de aceite

### C1 brainstorm

1. Com um brief e `egm.agent_cmd`, o `brainstorm` escreve um concept brief em `<state>/egm/<repo-id>/briefs/`, modo `0600`, com as quatro seções e a linha de status de gerado; a árvore do repositório não muda.
2. `--out` dentro de `egm.doc_globs` sai com 2 e `brainstorm_gated`; um `--out` existente sai com 2; nada é escrito.
3. Um brief com um valor parecido com segredo sai com 2 e `brief_secret`; um que dispara `ScanInjectionSource` sai com 2 e `brief_injection`; nenhum agente roda.
4. Sem `egm.agent_cmd`: `agent_unavailable`; com `tools_check` desligado: `tools_unverifiable`.
5. Um corpo com comentário HTML, H1 ou linha `Status:` é limpo; um que dispara o scan de injeção não escreve nada; um corpo sem uma seção duas vezes dá `brainstorm_invalid`.
6. `--reviewers brainstorm` é recusado.

### C2 draft (documento novo)

7. `draft --body` escreve um documento cujas fontes R7 são exatamente os arquivos `--from` aceitos, com a linha de status do draft e todas as seções obrigatórias; nenhum agente roda.
8. `draft --engine llm` faz o mesmo a partir do corpo do goldfish; `--body` com `--engine llm`, ou nenhum dos dois, sai com 2 e `usage`.
9. Um corpo que cita um arquivo fora de `--from` sai com 2 e `draft_sources`; um que dispara o scan de injeção sai com 2 e `draft_injection`; nada é escrito.
10. Depois do `draft`, `tb egm status` não mostra rodadas, `tb egm receipt` falha e não existe arquivo no diretório de estado do documento.
11. Um brief não rastreado não é citado no rascunho.
12. O tb nunca faz stage nem commit do rascunho.

### C3 draft `--update`

13. `--update` deixa o documento idêntico byte a byte, escreve uma proposta no diretório de estado e imprime um diff; uma proposta cujas fontes diferem do que o R7 resolve sai com 2 e `draft_sources`.

### C4 citações e verificação de símbolos

14. `GoSymbols` devolve funcs, métodos (`T.M` para `(*T[K]) M`), tipos, consts e vars com intervalos a partir de 1 e inclusivos; `init`, `_` e duplicados são ambíguos; um erro de parse dá `unparsed`.
15. Um documento que cita `x.go#Missing` recebe um achado `refs` `major` com `--engine deterministic`; `x.go#L10` continua uma citação por arquivo, sem achado de símbolo.
16. O estado da rodada registra `symbols` para citações resolvidas.

### C5 recibo v3 e drift por símbolo

17. Com `egm.receipt.write_v3: true`, o recibo é v3 com `symbols`; os leitores aceitam v1–v3; uma v3 com o conjunto de chaves errado é `receipt_invalid`.
18. No CI, um intervalo que edita o corpo de um símbolo citado de um documento com recibo e não tocado dispara `ci_source_drift`; um intervalo que edita só outra função no mesmo arquivo com escopo de símbolo não dispara nada, e o `drift-report` lista o caminho em `outside_symbols`, status `fresh`.
19. Apagar ou renomear um símbolo citado dá `symbol_missing` (`deleted`); uma edição só no comentário de documentação dá `fresh`.
20. Um arquivo citado sem fragmento e também com símbolo continua com escopo de arquivo: qualquer edição é `drifted`.
21. Um recibo v1/v2 dá exatamente o resultado da Fase B para a mesma árvore.
22. `drift-report --json` tem `v: egm-source-drift/0.2` e os três arrays novos em todo documento.
23. Duas execuções no mesmo commit dão JSON idêntico byte a byte.

### C6 grafo

24. Os nós Go de função, método e tipo levam `start_line`/`end_line`; dois métodos `Name` em receivers diferentes do mesmo pacote são dois nós com as próprias arestas de chamada.

## Slices

| Slice | Conteúdo | PR |
|---|---|---|
| **C1** (feito) | `tb egm brainstorm` (R1–R5), persona `brainstorm`, `egm.brainstorm.max_bytes`, docs | um PR |
| C2 (feito) | `tb egm draft` documento novo (R6–R10, R12), persona `draft`, `egm.draft.max_bytes` (D2: sem checker) | um PR |
| C3 (feito) | `draft --update` (R11) | um PR |
| C4 | `graph.GoSymbols`, citações de símbolo (R13–R17), verificação no `refs` | um PR |
| C5 | recibo v3, frescor no CI, status de drift (R18–R22), `docs/egm-ci.md` | um PR |
| C6 | intervalos de linha e IDs de método no grafo (R23–R24) | um PR, independente de C1–C5 |

C1 e C4 não dependem um do outro e podem andar em paralelo; o C5 precisa do C4; o C3 precisa do C2.

## Docs a atualizar com a implementação

Status do ADR-015 (§8 Fase C), Não-objetivos da [egm-nativo.md](egm-nativo.md), "Fora da Fase B" da [egm-drift-bootstrap.md](egm-drift-bootstrap.md) (link para cá), seção egm do `docs/cli.md` (e a seção do grafo no C6), [docs/egm-ci.md](../egm-ci.md) (C5: recibos v3, drift por símbolo), roadmap do README, CHANGELOG (C4: a forma nova de span; C6: IDs do grafo alterados) e o espelho pt-BR de cada um.

## Perguntas em aberto

1. **Status do ADR-015 vs item 8 do R0.** O [ADR-015](../adr/015-egm-nativo.md), linha 7, está `accepted` e também diz que o item 8 do R0 continua aberto; a [egm-nativo.md](egm-nativo.md), linha 124, diz que o item 8 "ainda bloqueia o ADR-015 de ir para `accepted`". **Resolvida, D7:** o R0 da egm-nativo agora diz que o item 8 é uma lacuna aberta conhecida que não bloqueia mais a aceitação; o ADR-015 segue `accepted`. A Fase C não depende do item 8.
2. **Status da egm-trust.** A [egm-trust.md](egm-trust.md) diz "Draft, round 2 … slices 1–3 done; the flakiness measurement is pending", o ADR-015 diz que a spec de confiança "continua rascunho" e o README diz que a Fase A.2 "está implementada". Resolvida: a egm-trust.md e o ADR-015 agora dizem "implementada (slices 1–3); medição de instabilidade pendente", como o README.
3. **`--ticket`.** O `brainstorm`/`draft` deve ler um `.toolbelt/tickets/TKT-NNN.yaml` direto, ou `--brief rd.md` basta? Fora do C1.
4. **Vários briefs ou rodadas de ideação.** Um goldfish por brainstorm mantém o orçamento limitado; um brainstorm com várias personas (divergir, depois consolidar) pode vir depois, se uma execução se mostrar estreita demais.
5. **pt-BR para rascunhos.** Neste repositório todo `docs/**/*.md` precisa de um gêmeo pt-BR (`TestDocsTranslationMirrorComplete`). O `draft` escreve só em inglês e imprime o caminho do espelho como lembrete (implementado); rascunhar a tradução fica fora.
