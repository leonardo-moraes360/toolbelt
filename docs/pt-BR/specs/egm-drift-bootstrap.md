[🇺🇸 English](../../specs/egm-drift-bootstrap.md) | 🇧🇷 Português

# `tb egm drift-report` e `tb egm bootstrap` — spec da Fase B

Status: **B1 implementado**; B2 e B3 não implementados (spec na rodada 2, depois de uma revisão em contexto novo).

Fase B do [ADR-015](../adr/015-egm-nativo.md) (§8). Apoia-se no núcleo ([egm-nativo.md](egm-nativo.md), Fase A) e na spec de confiança ([egm-trust.md](egm-trust.md), Fase A.2). Os nomes vêm do upstream, citado no [ADR-007](../adr/007-inspiracao-elephant-goldfish.md): `drift-report` "compara docs com o código atual" e `bootstrap` "documenta uma base de código existente".

A Fase B sai em três partes, cada uma usável sozinha:

- **B1:** `drift-report` determinístico, mais a checagem de CI que fecha a lacuna que a egm-trust.md deixa para a Fase B (drift só nas fontes, e edição de recibos).
- **B2:** `bootstrap` determinístico (um esqueleto com as fontes já ligadas).
- **B3:** as partes com LLM dos dois comandos, e o campo `commit` do recibo, de que só o diff do drift precisa.

## Problema

- **Drift só nas fontes passa despercebido.** Um documento revisado certifica o conteúdo e as fontes num momento (recibo e estado local). Quando uma fonte muda e o documento não, a op `egm` do `tb stack` só avisa `sources_changed`, a partir do estado local, e só para documentos revisados nesta máquina. O CI nunca vê. Uma spec pode descrever um código que não existe mais, e todos os gates continuam verdes.
- **Recibos podem mudar sem revisão.** Um pull request que só edita ou apaga `.toolbelt/egm/receipts/<slug>.json` hoje não seleciona nenhum documento com gate, então o CI não confere nada.
- **Código existente não tem documentos para passar pelo gate.** Um repositório que adota o `tb egm` tarde tem código, mas não tem specs nem ADRs que o citem. Escrever o primeiro rascunho à mão, com as fontes ligadas, é o passo que impede os times de começar.

## Princípios

1. **O tb decide, o LLM lê e julga.** O tb coleta, faz hash, compara e registra; qualquer chamada de modelo (só no B3) é um processo isolado, como um goldfish, sem tools.
2. **O tb nunca edita um documento existente.** O `drift-report` informa; o `bootstrap` só cria um arquivo novo.
3. **A base é o que o branch de destino commitou.** No CI, as checagens de drift e de recibo comparam com o recibo no merge-base, nunca com um recibo trazido pelo próprio pull request.
4. **Offline por padrão.** B1 e B2 nunca chamam um agente. O B3 é opt-in por execução (`--engine llm`).
5. **Texto gerado é dado.** Um rascunho do bootstrap não é confiável, vai para um arquivo novo marcado como gerado, nunca é commitado pelo tb e só conta depois de um `tb egm review` normal.

## Parte B1: `tb egm drift-report` e drift das fontes no CI

### Comando

```bash
tb egm drift-report                       # todo documento com gate
tb egm drift-report docs/specs/x.md       # alguns documentos (fora de egm.doc_globs: erro)
tb egm drift-report --at HEAD~3           # como estava num commit (padrão HEAD), a partir dos blobs
tb egm drift-report --json
```

A config do repositório em `--at` rege a comparação (globs e limites), como no gate da Fase A. O comando é só de CLI (nenhuma tool MCP na Fase B).

### Base

Para cada documento com gate em `--at`, a base é, nesta ordem:

1. o **recibo commitado** em `--at`, quando o `doc_sha256` dele é igual ao documento em `--at`;
2. senão, a **rodada do estado local** deste documento cujo `doc_sha256` é igual a ele **e cujo conteúdo e política têm veredito `PASS` ou `PASS_PARTIAL`** (as rodadas a partir das quais um recibo poderia ter sido escrito);
3. senão, **nenhuma**.

Uma base de outra versão do documento não é base: o próprio documento mudou, e isso é trabalho do `tb egm review` e do gate.

### Comparação

O drift compara **hashes dos caminhos da base**, não pacotes de revisão, então o orçamento do pacote nunca decide:

- Cada caminho de fonte da base é resolvido em `--at` com as regras de caminho da Fase A (dentro da raiz, sem symlink em nenhum componente, rastreado no índice com modo `100644` ou `100755`, não negado pela config em `--at`), **sem limite de tamanho**. Se resolver, o sha256 é comparado com o da base.
- À parte, todo candidato a fonte do documento (links e spans da R7) que resolve pelas mesmas regras de caminho e não está na base é **acrescentado**. Nem o orçamento do pacote nem as checagens de conteúdo decidem isso.
- Uma fonte da base acima de 256 MiB conta como mudada (o hash de uma base sempre vem de uma fonte R7 menor).

| Status | Significado | Saída |
|---|---|---|
| `fresh` | todo caminho da base resolve com o mesmo sha256, e nada foi acrescentado | 0 |
| `no_baseline` | nenhum recibo ou rodada convergida bate com esta versão do documento | 0 |
| `sources_differ` | os caminhos da base não mudaram, mas um caminho que passou a resolver foi acrescentado | 1 |
| `drifted` | ao menos um caminho da base resolve com outro sha256 (`changed`) | 1 |
| `source_missing` | um caminho da base não resolve mais (`missing`, cada um com o motivo: `deleted`, `symlink` (o caminho ou um diretório acima dele), `not_regular`, `denied`) | 1 |
| `unreadable` | o documento ou o recibo não puderam ser lidos em `--at` | 2 |

Um documento recebe o status mais grave (`unreadable` > `source_missing` > `drifted` > `sources_differ` > `fresh` > `no_baseline`), e todas as listas são informadas. O código de saída do comando é o maior entre os documentos; erros de uso saem com 2, como a R32 define (`ExitError`).

### Saída

O texto agrupa os documentos por status, do mais grave ao menos grave. `--json` (ordem de chaves estável; `reason` só aparece em `unreadable`):

```json
{"at": "<commit>", "documents": [
  {"added": [], "baseline": "receipt", "changed": ["internal/x/x.go"], "doc": "docs/specs/x.md",
   "missing": [], "status": "drifted"}
], "v": "egm-source-drift/0.1"}
```

`baseline` é `receipt`, `state` ou `none`; os itens de `missing` são `{"path", "reason"}`.

### CI: edição de recibos e drift das fontes

O `--egm-mode ci` ganha duas checagens. As duas usam o **recibo no merge-base** (`merge-base(base, head)`), então nada que o pull request commita pode servir de base para ele mesmo.

1. **Edições de recibo selecionam o documento.** Um intervalo que acrescenta, altera ou apaga `.toolbelt/egm/receipts/<slug>.json` seleciona o documento com gate desse recibo para a checagem completa da Fase A.2 (T-R2: recibo em head, uma rodada nova, alegações). Apagar o recibo de um documento com gate que ainda existe, e que tinha recibo no merge-base, falha com `receipt_removed`.
2. **`ci_source_drift`.** Para cada documento com gate que o intervalo não tocou, cujo recibo no merge-base bate com o conteúdo dele no merge-base e estava `fresh` lá, a comparação acima roda em head contra esse recibo do merge-base. Um documento que fica `drifted`, `source_missing` ou `sources_differ` neste intervalo gera `ci_source_drift`. Documentos que já tinham drift no merge-base não culpam este intervalo.
   - A gravidade é `egm.drift.ci`: `warn` (padrão) ou `fail`. É uma chave de política: a config do repositório só pode subi-la para `fail`.
   - O jeito de resolver é atualizar o documento ou revisá-lo de novo; a checagem 1 então exige uma revisão completa no CI para o recibo novo.
   - As duas comparações usam as configurações do **merge-base** (a config de usuário `-c` mais a config do repositório no branch de destino), então a config do próprio pull request não consegue baixar um limite para esconder o drift.

O custo é uma coleta de fontes por documento com gate no merge-base e em head, sem rodar agente; o `egm.ci.max_docs` não vale aqui (ele limita execuções de agente).

### Gate local

A op `egm` do `tb stack` mantém o código de aviso `sources_changed` (consumidores o interpretam), agora calculado pela comparação acima para documentos cujos caminhos da base mudaram no intervalo da stack: primeiro pelos recibos, e pelas rodadas convergidas do estado quando não há recibo. Continua sendo um aviso; um documento cujo drift não pode ser calculado recebe `drift_unavailable` em vez de sumir.

## Parte B2: `tb egm bootstrap`

### Comando

```bash
tb egm bootstrap docs/specs/router.md --from internal/router/
tb egm bootstrap docs/adr/020-x.md --from internal/x/x.go --from cmd/tb/x.go
```

- **Destino:** não pode existir (`O_CREAT|O_EXCL`), precisa casar com `egm.doc_globs`, e o diretório pai precisa existir dentro do repositório, sem symlink em nenhum componente (como faz o `tb vault new`).
- **Tipo:** derivado do `egm.doc_types` para o caminho do destino, exatamente como o `required-sections` vai derivá-lo na revisão. Não há flag `--type`.
- **`--from`:** arquivos ou diretórios dentro do repositório, ao menos um.
  - Um diretório se expande nos arquivos rastreados dele (`git ls-files` dentro dele); arquivos não rastreados nunca entram.
  - Cada arquivo precisa passar nas regras de caminho da Fase A (sem symlink em nenhum componente, regular, rastreado, não negado). Um arquivo que não passa é pulado e impresso com o motivo; ele não é escrito no rascunho.
  - No B2 o tb não lê o conteúdo de nenhum arquivo.

### Esqueleto

O tb escreve, sem modelo:

- um título a partir do nome do arquivo de destino;
- a linha de status `Status: **draft, generated by tb egm bootstrap** (not reviewed).`;
- toda seção que o padrão derivado exige (a lista que o `required-sections` confere), cada uma com uma linha `TODO`;
- uma seção `## Sources` que lista cada arquivo aceito como **link Markdown relativo ao documento** (links resolvem qualquer nome de arquivo; spans em crase precisam de uma extensão conhecida).

Antes de criar o arquivo, o tb roda a coleta R7 no texto montado. Se o conjunto incluído não for exatamente o dos arquivos aceitos (um arquivo cortado como `secret_like`, ou por `max_source_bytes`/`max_packet_bytes`), o tb recusa com `bootstrap_sources` e diz o motivo. Assim, a revisão do rascunho nunca começa com uma fonte excluída, o que seria um rebaixamento que o CI reprova como `ci_partial`.

### Depois do bootstrap

O tb imprime os próximos passos: editar o rascunho e rodar `tb egm review <destino>`. O rascunho não tem recibo nem estado, então o `drift-report` o lista como `no_baseline`. O tb nunca o coloca no stage nem o commita.

## Parte B3: partes com LLM

### `commit` do recibo (só auxiliar)

- Os recibos podem trazer `commit`: o `HEAD` no momento em que foram escritos. Ele é **só auxiliar**: serve para montar o diff do drift, nunca para julgar um recibo.
- O tb só o usa quando o objeto existe e toda fonte do recibo em `commit` tem o hash que o recibo registra (e o documento, o `doc_sha256`). Senão o relatório diz `no_diff` e usa o conteúdo atual.
- **Implantação:** os leitores aceitam `version` 1 e 2 (a 2 exige um `commit` de 40 ou 64 caracteres hex); o tb só escreve a versão 2 com a chave de config só do usuário `egm.receipt.write_v2: true`, para que um CI fixado num tb mais antigo seja atualizado antes.

### Goldfish de drift (`tb egm drift-report --engine llm`)

O `--engine` aceita `deterministic` (o padrão aqui, não `auto`) e `llm`.

Para cada documento `drifted`, até `egm.drift.max_docs` (padrão 5, uma chave de limite: o repositório só pode baixá-la), roda um goldfish de drift com o isolamento da Fase A (sem tools, sandbox, HOME descartável, ambiente mínimo, blocos DATA delimitados por nonce) e o schema de findings da Fase A. O pacote dele:

- a persona embutida `drift`: *cada afirmação deste documento sobre estas fontes ainda vale?*;
- o documento;
- cada fonte que mudou, **inteira em `--at`** (para que as linhas de evidência se confiram contra ela), dentro dos limites da R7;
- o diff unificado de cada fonte que mudou, de `commit` até `--at`, quando o `commit` é utilizável.

Os dois lados de todo diff são montados a partir da saída do `Redact`; uma fonte `secret_like` em qualquer um dos lados fica de fora, com o motivo; os dois lados passam pelo `ScanInjectionSource`. Os diffs vêm do git endurecido do tb: `CatFileBlob` para cada lado e uma variante do `DiffNoIndex` com linhas de contexto, num diretório temporário privado (`--no-ext-diff --no-textconv`).

Os findings são **informados, não guardados** como linhagens de revisão, no `findings` de cada documento. Eles não mudam o código de saída. Cada execução conta para o `egm.max_agent_runs` e o `max_stdout_bytes`. Um perfil de vendor em que o uso de tools não pode ser verificado (`tools_check` desligado) é recusado com `tools_unverifiable`; `--engine llm` sem `egm.agent_cmd` falha com `agent_unavailable`.

### Rascunho do bootstrap (`tb egm bootstrap --engine llm`)

Roda um goldfish de bootstrap, isolado do mesmo jeito, com a persona embutida `bootstrap` (descrever o que o código faz; deixar intenção e decisões como perguntas em aberto explícitas), o padrão derivado e os arquivos aceitos do `--from`, lidos como fontes R7 (do índice, com `Redact`, `secret_like` pulado e `ScanInjectionSource`). O contrato de saída é um corpo Markdown simples de até `egm.bootstrap.max_bytes` (padrão 32 KiB).

O tb monta o arquivo como o esqueleto, com o corpo do modelo no lugar das seções `TODO`, e então:

- tira do corpo todo comentário HTML (inclusive marcadores do tb como `<!-- tb:egm-example -->`), todo heading de nível 1, toda linha que começa com `Status:` antes do primeiro `##` e qualquer seção `## Sources`;
- roda o `injection-scan` da Fase A no corpo; um finding aborta com `bootstrap_injection`;
- roda a coleta R7 no arquivo montado; um conjunto de fontes diferente dos arquivos aceitos (o corpo citou outra coisa) aborta com `bootstrap_sources`;
- recusa um corpo vazio, grande demais ou fora de UTF-8 com `bootstrap_invalid`.

Nada é escrito quando aborta.

### Personas

`drift` e `bootstrap` são embutidas, mas ficam fora da lista de reviewers: o `egm.reviewers` e o árbitro as recusam.

## Fora da Fase B

- Abrir linhagens de revisão a partir de findings de drift, ou recertificar automaticamente.
- Fazer bootstrap de vários documentos de uma vez, ou escolher os arquivos sozinho.
- Tools MCP para os comandos novos.
- `draft` e `brainstorm`, drift por símbolo e os outros itens da Fase C.

## Docs a atualizar com a implementação

Status do ADR-015, Non-Goals da [egm-nativo.md](egm-nativo.md), [egm-trust.md](egm-trust.md) (as duas menções à Fase B), seção egm do `docs/cli.md`, roadmap do README, CHANGELOG e o espelho pt-BR de cada um.

## Critérios de aceite

### B1

1. Um documento com recibo num commit, cuja fonte citada muda depois, fica `drifted` com esse caminho em `changed`, base `receipt`; saída 1.
2. Uma fonte apagada, ou que virou symlink, fica `source_missing` com o motivo; um caminho citado que passou a resolver fica `sources_differ`; os dois saem com 1.
3. Uma fonte que passa de `max_source_bytes` continua sendo comparada por hash (`drifted` ou `fresh`), não é informada como ausente; outra fonte crescer nunca muda isso.
4. Um documento cujo próprio texto mudou depois do recibo fica `no_baseline`; saída 0.
5. Sem recibo, só uma rodada convergida em `PASS`/`PASS_PARTIAL` serve de base (`state`); uma rodada `FAIL` não serve.
6. `--at <commit>` lê os blobs e a config daquele commit, sem mexer no worktree.
7. Um documento explícito fora de `egm.doc_globs`, e um recibo ilegível, saem com 2.
8. No CI, um intervalo que só reescreve os sha256 de um recibo seleciona o documento dele para a checagem completa; um intervalo que apaga o recibo de um documento que ainda tem gate falha com `receipt_removed`.
9. No CI, um intervalo que muda uma fonte citada de um documento intocado e fresh gera `ci_source_drift`, como aviso por padrão e como falha com `egm.drift.ci: fail`; a config do repositório não consegue baixar `fail` para `warn`.
10. Um documento que já tinha drift no merge-base não gera `ci_source_drift` num intervalo sem relação.
11. O gate local continua emitindo `sources_changed`, agora também para documentos com recibo que nunca foram revisados nesta máquina.

### B2

12. Um bootstrap escreve o título, a linha de status de gerado, toda seção que o padrão derivado exige e uma seção `## Sources` com links relativos; as fontes R7 do arquivo novo são exatamente os arquivos aceitos do `--from`.
13. Recusados sem escrever nada: um destino que já existe, um destino fora de `egm.doc_globs`, um symlink no caminho do destino, um `--from` fora do repositório.
14. Um diretório no `--from` com um arquivo não rastreado, um arquivo de `testdata`, um arquivo de segredos negado e um componente em symlink não lista nenhum deles e imprime cada arquivo pulado com o motivo.
15. Um arquivo do `--from` que a R7 excluiria (conteúdo com cara de segredo, acima dos limites) recusa o bootstrap com `bootstrap_sources`.
16. O tb nunca coloca o rascunho no stage nem o commita.

### B3

17. Um recibo v2 com `commit` utilizável dá um diff ao goldfish de drift; um recibo v1, um objeto ausente ou um `commit` cujos blobs não batem dão `no_diff`, nunca `receipt_invalid`.
18. Os leitores aceitam v1 e v2; o tb só escreve v2 com `egm.receipt.write_v2: true`.
19. Um diff cujo lado antigo tem um valor com cara de segredo deixa essa fonte fora do pacote.
20. Os findings de drift aparecem no relatório e nada é escrito no estado de revisão; rodam no máximo `egm.drift.max_docs` goldfish.
21. Um corpo do bootstrap com comentário HTML, heading de nível 1, linha `Status:`, seção `## Sources` ou citação de outro arquivo é limpo ou recusado como especificado; um que dispara o injection scan não escreve nada.
22. `tools_check` desligado recusa `--engine llm` com `tools_unverifiable`; o `egm.reviewers` recusa `drift` e `bootstrap`.
