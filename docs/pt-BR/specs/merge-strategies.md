[🇺🇸 English](../../specs/merge-strategies.md) | 🇧🇷 Português

# Estratégias `semantic` e `prompt` do `tb merge` — spec

Status: **rascunho**. Nada aqui está implementado. Fecha o item do roadmap do README "`tb merge` `prompt` / `semantic` strategies".

## Visão geral

O `tb merge` integra tickets `done` num worktree temporário e só move a ref alvo no fim, com compare-and-swap. Hoje um conflito aborta o merge (`fail`) ou mantém o arquivo inteiro do ticket mais recente (`last_wins`). O `last_wins` descarta em silêncio as edições do ticket anterior nesse arquivo.

Esta spec adiciona duas estratégias. Elas formam uma escada: cada uma faz primeiro o trabalho da anterior.

| Estratégia | Quem resolve | LLM | Avança refs numa chamada |
|------------|--------------|-----|--------------------------|
| `semantic` | o tb, com regras determinísticas sobre os hunks que o git deixou em conflito | não | sim, como `fail`/`last_wins` |
| `prompt` | `semantic` primeiro, depois um agente one-shot em sandbox para os hunks restantes | sim (processo filho) | **não**: propõe um recibo; uma segunda chamada o aplica |

As duas recusam em vez de adivinhar: um hunk que nenhuma regra (nem nenhuma resposta válida do agente) resolve deixa o merge em conflito, e nenhuma ref muda.

## Problema

- **Tickets paralelos conflitam em edições triviais.** Dois tickets que adicionam imports diferentes ao mesmo arquivo Go, ou editam linhas adjacentes, conflitam. O git recusa edições adjacentes de propósito. O `fail` então bloqueia o merge inteiro.
- **O `last_wins` perde trabalho.** Ele substitui o arquivo inteiro (`internal/merge/merge.go:503-505`), então as edições do ticket anterior nesse arquivo somem, sem relatório por hunk.
- **Nenhum design registrado.** `prompt` e `semantic` são nomes reservados que falham com "not implemented". Nenhum doc diz o que devem fazer nem até onde uma resolução escrita por agente é confiável.

## Estado atual

| O quê | Onde |
|-------|------|
| Constantes `Strategy`: só `fail` e `last_wins` | `internal/merge/merge.go:23-28` |
| `ParseStrategy` rejeita `prompt`/`semantic` como "not implemented" | `internal/merge/merge.go:31-42` |
| `Options` (`DryRun`, `Output`, `AllowSensitive`, …) | `internal/merge/merge.go:47-60` |
| `File`, `Conflict` (`Resolution` é `""` ou `last_wins`), `Result` | `internal/merge/merge.go:62-96` |
| `Run`: gate de caminhos sensíveis, worktree temporário, `pickAll`, `finalize` | `internal/merge/merge.go:165-217` |
| `checkSensitive` (recusa só ao aplicar) | `internal/merge/merge.go:242-255` |
| `validateOptions` repete a checagem de estratégia | `internal/merge/merge.go:257-272` |
| `pickAll` para no primeiro conflito com `fail` | `internal/merge/merge.go:325-347` |
| `pick`: um `cherry-pick` por commit; no conflito lê `--diff-filter=U` | `internal/merge/merge.go:506-552` |
| `resolveTheirs`: arquivo inteiro do commit (`last_wins`) | `internal/merge/merge.go:557-570` |
| `finishPick`: `--skip` quando vazio, senão `--continue` | `internal/merge/merge.go:575-582` |
| `advance`: ff-only num worktree com checkout, senão CAS via `update-ref` | `internal/merge/merge.go:585-610` |
| Args de git endurecidos (sem hooks, fsmonitor, diff externo, pager, assinatura) | `internal/merge/sensitive.go:20-29` |
| Caminhos sensíveis (diretórios de hooks, `.toolbelt/`, workflows, `.gitmodules`, `.gitattributes`, gitlinks) mais os caminhos protegidos do guard | `internal/merge/sensitive.go:86-107`, `:111-146` |
| Flags da CLI e texto de ajuda ("prompt/semantic are not implemented") | `cmd/tb/merge.go:15-40` |
| MCP `tb_merge`: dry-run salvo `apply:true`; conflito devolvido com `ok:false` | `internal/registry/merge.go:10-61` |
| Runner de agente one-shot em sandbox (Landlock, subreaper, HOME descartável, teto de stdout, checagem de uso de ferramentas) | `internal/egm/run.go:114` (`NewRunner`), `:227` (`Run`), `:124-161` (`classify`) |
| Perfil do agente: `egm.agent_cmd[0]` absoluto, flags proibidas, `tools_disabled` para vendors custom | `internal/egm/vendor.go:48-96` |
| Leitura só da config de usuário (nunca o arquivo do repositório) | `internal/egm/config.go:32-48` |
| Redação de segredos e lista de negação | `internal/secrets/secrets.go:32`, `:58` |
| O router nunca chama o merge | `internal/router/` (nenhuma referência) |

## Objetivos

- G1: o `semantic` resolve os conflitos comuns e mecânicos de tickets paralelos sem LLM, de forma determinística, e relata cada hunk resolvido.
- G2: o `prompt` resolve os hunks restantes com um agente que só pode mudar os hunks em conflito, não toca o disco, roda em sandbox e tem a resposta validada antes do uso.
- G3: uma resolução escrita por agente nunca move uma ref na chamada que a produziu. Um humano revisa um recibo, e a etapa de aplicação o reproduz sem chamar o agente.
- G4: trilha de auditoria completa: por hunk, a regra ou execução do agente que o resolveu, os hashes das entradas e a árvore resultante.
- G5: nada muda para quem já usa. `fail` continua o padrão, e a saída e o JSON de `fail`/`last_wins` ficam idênticos byte a byte.

## Não-objetivos

- Auto-merge pelo router. O router continua sem chamar o merge.
- Resolver conflitos em caminhos sensíveis, arquivos binários, symlinks, gitlinks, nem conflitos de delete/modify, rename e modo.
- Merge em nível de AST para linguagens além de Go, ou um motor de merge estrutural geral (estilo mergiraf).
- Rodar comandos de build/teste sobre a árvore integrada nas fases A ou B (ver Fase C e Questões em aberto).
- Expor o `prompt` pelo MCP na Fase B.
- Qualquer acesso à rede pelo tb. O filho do agente pode usar a rede que o humano configurou para ele; o tb não abre nenhuma.

## Design

### Modelo de conflito

Os conflitos continuam sendo encontrados pelo `cherry-pick` por commit no worktree temporário (`pick`, `merge.go:506`). Para cada caminho não mesclado, o tb lê os estágios do índice com `git ls-files -u -z` (modo e blob dos estágios 1, 2 e 3) e classifica o arquivo:

- **recusas por arquivo** (motivo entre parênteses): estágio 1 ausente (`add_add`), estágio 2 ou 3 ausente (`delete_modify`), estágios com modos diferentes (`mode`), modo `120000` (`symlink`) ou `160000` (`gitlink`), byte NUL nos primeiros 8000 bytes de algum estágio (`binary`), caminho sensível (`sensitive`, ver M14), estágio maior que `merge.semantic.max_file_bytes` (`too_large`);
- **arquivos de texto**: o tb grava os três blobs num diretório temporário privado (0700) e roda `git merge-file -p --diff3 --marker-size=<n> -L ours -L base -L theirs` com os args endurecidos. `<n>` é um a mais que a maior sequência de `<`, `=`, `>` ou `|` no início de qualquer linha dos três estágios, então o parser do tb nunca confunde conteúdo do arquivo com marcador. A saída é dividida em texto limpo e **regiões de conflito**, cada uma com suas linhas de base, ours e theirs.

O `git merge-file` não lê atributos nem roda merge driver, então nenhuma config do repositório consegue executar código aqui.

"Ours" é a integração acumulada (`HEAD` do worktree temporário), "theirs" é o commit do ticket sendo aplicado, e "base" é a versão do pai desse commit.

### Requisitos

**Parsing da estratégia e opções**

- M1: `ParseStrategy` aceita `semantic` e `prompt` (minúsculas, sem espaços). `validateOptions` aceita as quatro estratégias. O texto de erro para estratégias desconhecidas lista as quatro.
- M2: `Options` ganha `Resolutions string` (um ID de recibo, só `prompt`) e `Config string` (o override `-c` da config de usuário, do comando raiz). `--resolutions` com qualquer outra estratégia é erro de uso.

**`semantic` (determinístico, Fase A)**

- M3: Para cada região de conflito, o tb tenta as regras abaixo em ordem; a primeira que se aplica resolve a região. Um arquivo só fica resolvido quando todas as suas regiões ficam; o resultado é o texto limpo com cada região trocada pela sua resolução, preparado com `git add`. Senão o arquivo fica sem resolução, com um motivo por região.
- M4: Regra `disjoint`: o tb faz o diff de ours contra base e de theirs contra base dentro da região (LCS em Go puro, limitado por `merge.semantic.max_region_lines`, senão motivo `region_too_large`). Quando os dois lados mudam faixas de linhas da base disjuntas, a resolução aplica os dois conjuntos de edições. Duas inserções na mesma posição da base, ou edições que tocam uma linha da base em comum, são motivo `overlap`.
- M5: Regra `go_imports`: para um arquivo `.go` cujos estágios ours e theirs fazem parse (`go/parser`, `ImportsOnly`), uma região cujas linhas de base, ours e theirs estão todas dentro da mesma declaração `import` entre parênteses, e cada uma é uma única import spec (sem linhas em branco e sem comentários), resolve para o conjunto ours ∪ theirs menos as specs que algum lado removeu da base, ordenado pelo caminho do import, um por linha, com a indentação de ours. Qualquer outra coisa na região passa para a próxima regra.
- M6: Pós-checagem de um arquivo resolvido: (a) nenhuma região resolvida tem uma linha que seja marcador de conflito de tamanho 7 ou `<n>` (`<<<<<<<`, `|||||||`, `=======`, `>>>>>>>`, sozinho ou seguido de espaço), salvo se essa mesma linha aparece na base, ours ou theirs da região; (b) um arquivo `.go` cujos estágios ours e theirs fazem parse precisa fazer parse depois da resolução (`go/parser`, arquivo inteiro), senão fica sem resolução com motivo `parse_failed`. A Fase A não checa outras linguagens.
- M7: Quando algum arquivo de um pick fica sem resolução, o `semantic` se comporta como `fail`: `cherry-pick --abort`, para, nenhuma ref muda. O erro embrulha `ErrConflict` e sugere `--strategy prompt` ou `--strategy last_wins`.
- M8: O `semantic` não precisa de flag extra para aplicar: quando todo conflito se resolve, finaliza como `fail`/`last_wins` (dry-run, `--output` e o gate de caminhos sensíveis inalterados). Ver DECISÃO DO USUÁRIO 2.

**Resultado e JSON (Fase A)**

- M9: `Conflict` ganha `reason` (vazio quando resolvido) e `hunks` (por região: `region`, `ours_lines`, `theirs_lines` como `[start,end]` no arquivo do estágio, `reason` ou `rule`). `Conflict.Resolution` ganha `semantic` e `prompt`. `File.Status` ganha `semantic` e `prompt`. Com `fail`/`last_wins` os campos novos são omitidos (`omitempty`), então o JSON deles não muda.
- M10: `Result` ganha `resolutions` (omitido quando vazio): uma entrada por região resolvida com `file`, `ticket`, `commit`, `region`, `rule` (`disjoint`, `go_imports` ou `agent`), `base_sha256`, `ours_sha256`, `theirs_sha256` (das linhas da região) e `result_sha256`. `Result` também ganha `result_tree` (o SHA da árvore do commit integrado) sempre que `result_sha` estiver preenchido.
- M11: A tabela de texto imprime `resolved by semantic (disjoint)` e afins por conflito, e `unresolved (<reason>)` com as faixas de linhas das regiões caso contrário.

**`prompt` (agente, Fase B)**

- M12: O `prompt` roda o `semantic` primeiro. Só as regiões que o `semantic` deixou sem resolução vão para o agente. Um arquivo com recusa por arquivo (binário, symlink, sensível, …) nunca vai para o agente.
- M13: **Duas etapas.** `tb merge --strategy prompt <ids>` (a proposta) integra, pergunta ao agente, valida, grava um recibo (M20) e **nunca avança uma ref**, diga o `--dry-run` o que disser. Imprime o ID do recibo, seu caminho e o `result_tree`, e respeita `--output` (o patch da proposta). `tb merge --strategy prompt --resolutions <id> <ids>` (a aplicação) reintegra, roda o `semantic`, pega do recibo o texto de cada região restante, **não inicia o agente** e então finaliza como as outras estratégias. Ver DECISÃO DO USUÁRIO 1.
- M14: **Nunca enviado ao agente**, motivo entre parênteses: caminho sensível (`sensitive`, o mesmo predicado do `checkSensitive`, mesmo com `--allow-sensitive`); caminho na lista `secrets.Denied` (`secret_path`); arquivo cujo texto de prompt (regiões mais contexto) tem um match de `secrets.Redact` (`secret_in_context`). O tb não redige e envia: um hunk redigido não pode ser resolvido corretamente, e o invariante proíbe segredos em prompts.
- M15: **O agente.** O tb inicia um `egm.Runner` por merge (`egm.NewRunner`, o mesmo sandbox Landlock, subreaper, HOME descartável e perfil de vendor do motor llm do egm), configurado só pela config de usuário (M23, M24). Uma execução goldfish por arquivo em conflito, com todas as suas regiões sem resolução. O workdir é um diretório de pacote novo, vazio e somente leitura; o prompt vai pelo stdin. O agente não consegue ler o repositório nem gravar nada: o tb faz o splice da resposta.
- M16: **Prompt.** Um template fixo no binário do tb, com um nonce de 128 bits. O conteúdo não confiável (IDs e títulos dos tickets, caminho do arquivo, base/ours/theirs de cada região e até `context_lines` linhas de ours antes e depois dela) fica dentro de blocos delimitados pelo nonce, e as instruções dizem que esse conteúdo é dado. O template pede exatamente um objeto JSON: `{"nonce":"<nonce>","regions":[{"id":<k>,"text":"<resolved lines>"}]}`, ou `{"nonce":"<nonce>","refuse":"<reason>"}`.
- M17: **Validação** do payload (qualquer falha deixa o arquivo sem resolução com o motivo entre parênteses): o JSON faz parse e o nonce confere (`agent_protocol`); os IDs de região são exatamente os enviados, cada um uma vez (`agent_protocol`); nenhum texto tem byte NUL ou UTF-8 inválido (`agent_protocol`); cada texto tem no máximo `4 × (bytes de ours + theirs) + 4 KiB` (`agent_oversized`); as checagens do M6 passam (`agent_markers`, `parse_failed`); `secrets.Redact` não encontra num texto nada que não encontrou em ours ou theirs da região (`agent_secret`). Uma resposta `refuse` é motivo `agent_refused`.
- M18: **Falhas de execução.** `timeout`, `missing_output` e `exit_<n>` têm uma nova tentativa (`merge.prompt.attempts`, padrão 2); depois o arquivo fica sem resolução com esse motivo. `tools_used`, `escaped_process`, `unattributed_orphan` e `sandbox_error` não têm nova tentativa e abortam o merge inteiro (sem recibo, sem mudança de ref), porque o isolamento quebrou. `interrupted` (SIGINT, orçamento total) aborta do mesmo jeito.
- M19: **Linhas novas.** Para cada região do agente, o tb conta as linhas resolvidas que não aparecem em base, ours nem theirs (`novel_lines`). Elas aparecem no JSON, na tabela e no recibo. Com `merge.prompt.max_novel_lines` definido, uma região acima dele fica sem resolução (`agent_novel`). O padrão é indefinido (só relata). Ver DECISÃO DO USUÁRIO 4.

**Recibo e aplicação (Fase B)**

- M20: **Recibo.** Um arquivo JSON gravado uma vez (0600, atômico com temporário mais rename) em `$XDG_STATE_HOME/toolbelt/merge/<repo hash>/` (diretório 0700; `TB_MERGE_STATE_DIR` o substitui em testes). Fica fora do repositório e fora do diretório do git, então um worker ou um ticket não consegue plantar um. Seu ID são os primeiros 16 caracteres hex do seu sha256. Contém:
  - `v`, `created_at`, `target`, `start_sha`, `strategy`;
  - `tickets`: `id`, `base`, `head` por ticket, na ordem de aplicação;
  - `agent`: `vendor`, `output`, `argv_sha256` (`egm.ArgvSHA`) e a versão do tb;
  - `runs`: por arquivo e tentativa, `reason`, `duration_ms`, `prompt_sha256`, `payload_sha256` (sem stderr, sem ambiente);
  - `resolutions`: as entradas do M10, mais `text` e `novel_lines` nas entradas `agent`;
  - `unresolved`: os arquivos e regiões que ficaram em conflito, com motivos;
  - `result_tree`: a árvore da integração proposta, vazia quando algo ficou sem resolução.

  Uma proposta com regiões sem resolução ainda grava o recibo (para auditoria), mas sai com `ErrConflict`, e esse recibo não pode ser aplicado (`receipt_incomplete`).
- M21: **Checagens da aplicação**, todas antes de qualquer ref se mover (motivo entre parênteses): o sha256 do arquivo confere com o ID (`receipt_tampered`); `target`, `start_sha` e a lista de tickets (IDs, bases, heads, ordem) conferem com esta execução (`receipt_stale`, que diz para rodar a proposta de novo); toda região que o `semantic` deixa sem resolução tem uma entrada `agent` com `file`, `commit`, `region` e hashes de entrada iguais (`receipt_mismatch`); a árvore integrada é igual a `result_tree` (`receipt_mismatch`). Depois roda a finalização normal: o gate de caminhos sensíveis, `--dry-run`, `--output`, o avanço por CAS e o `merged_at`.
- M22: `tb merge receipts` fica fora da Fase B; os recibos são lidos com `cat`/`jq` a partir do caminho impresso. Recibos com mais de 30 dias são apagados na próxima execução do `prompt`.

**Configuração (Fase A para `semantic`, Fase B para `prompt`)**

- M23: Toda chave nova é lida **só da config de usuário** (o arquivo do `-c`, senão a config global), via `config.LoadUser` (o `LoadUserConfig` generalizado, como faz a spec do índice vetorial). Um `.toolbelt/config.yaml` do repositório que defina qualquer chave `merge.*` recebe um aviso `config_untrusted` e o valor é ignorado.
- M24: O agente é o agente do egm: `egm.agent_cmd`, `vendor`, `output`, `env.pass`, `home.*`, `path`, `tools_disabled`, `tools_check` e `forbidden_flags`, validados por `egm.ResolveProfile`. Não existe um segundo `agent_cmd`. Ver DECISÃO DO USUÁRIO 5.

| Chave | Padrão | Significado |
|-------|--------|-------------|
| `merge.semantic.max_region_lines` | `2000` | Acima disso a região é `region_too_large` |
| `merge.semantic.max_file_bytes` | `1048576` | Acima disso o arquivo é `too_large` |
| `merge.prompt.max_files` | `10` | Mais arquivos em conflito que isso para o agente: recusa antes de qualquer execução (`prompt_budget`) |
| `merge.prompt.context_lines` | `20` | Linhas de ours antes e depois de cada região |
| `merge.prompt.max_prompt_bytes` | `262144` | Por arquivo; acima disso o arquivo fica sem resolução (`prompt_budget`), nunca truncado |
| `merge.prompt.timeout_per_file` | `5m` | Por execução goldfish |
| `merge.prompt.max_total` | `20m` | Proposta inteira; passado isso a execução é `interrupted` |
| `merge.prompt.attempts` | `2` | Por arquivo, só falhas transitórias (M18) |
| `merge.prompt.max_stdout_bytes` | `1048576` | Repassado ao runner |
| `merge.prompt.max_novel_lines` | indefinido | M19 |

**Plataforma e sandbox (Fase B)**

- M25: O `prompt` precisa do mesmo que o motor llm do egm (Linux, Landlock, subreaper). Sem eles o `NewRunner` falha e o merge para com esse motivo (`no_landlock`, `no_subreaper`) antes de integrar. Não há fallback para `semantic` nem para execução sem sandbox.
- M26: Sem `egm.agent_cmd`, o `prompt` falha com `agent_unavailable`. O `semantic` nunca lê a config do egm.

**CLI e MCP**

- M27: CLI: a ajuda de `--strategy` lista `fail|last_wins|semantic|prompt`; nova flag `--resolutions <id>`; o override da config de usuário é a flag raiz `-c/--config` (`cmd/tb/main.go:68`). A ajuda longa troca "prompt/semantic are not implemented" por um resumo do M3–M8 e do M13.
- M28: MCP `tb_merge`: `strategy` aceita `semantic` (dry-run salvo `apply:true`, como hoje). `prompt` é recusado com `prompt_cli_only` na Fase B: quem chama pelo MCP costuma ser um agente que escreveu os tickets, e não deve iniciar um segundo agente nem aplicar um recibo. Sem parâmetro `resolutions`. Ver DECISÃO DO USUÁRIO 3.

### Por que essas escolhas

- **`semantic` sobre o `git merge-file`, não um diff3 novo.** O git já mesclou tudo o que conseguiu; o tb só acrescenta regras para as regiões que o git deixou. Um diff3 em Go puro poderia discordar do git sobre o que conflita, e seria mais um motor de merge em que confiar.
- **Splice, não respostas de arquivo inteiro.** O agente devolve textos de região e o tb grava o arquivo, então "só os hunks em conflito mudaram" vale por construção, não por uma checagem depois.
- **Duas etapas no `prompt`.** O agente leu conteúdo que outros agentes escreveram (os tickets) e pode ser guiado por ele. Um humano lendo o recibo é o avaliador em contexto novo que o invariante "implementador e avaliador são separados" pede. A aplicação reproduz bytes que foram revisados, amarrados pelos hashes de entrada e pela árvore resultante.
- **Recusar em vez de redigir.** Uma região redigida não pode ser resolvida corretamente, e o invariante diz que segredos nunca chegam a um prompt.

#### Alternativas consideradas

| Alternativa | Por que não (agora) |
|-------------|---------------------|
| `semantic` = `git merge-file` puro por arquivo | Reproduz os próprios conflitos do cherry-pick; não acrescenta nada |
| Regra de união (`merge=union`: mantém as linhas dos dois lados) no `semantic` | Duplica ou reordena código em silêncio; fica como questão em aberto para regiões só de acréscimo |
| Merge estrutural ciente da linguagem para várias linguagens | Grande, um parser por linguagem; fora do escopo; Go é a linguagem do próprio repositório |
| `prompt` grava arquivos numa cópia gravável do worktree | Exige uma pós-checagem de que só os arquivos em conflito mudaram e alarga o sandbox; o splice é mais simples e mais forte |
| `prompt` aplica numa chamada atrás de uma flag (`--accept-agent`) | Pula a revisão em que o modelo de confiança se apoia; ver DECISÃO DO USUÁRIO 1 |
| Redigir segredos e enviar | Resoluções erradas, e enfraquece o invariante "segredos nunca em prompts" |
| Uma segunda chave `merge.agent_cmd` | Duas configs de agente para endurecer e manter em sincronia; o runner do egm já impõe o perfil |
| Recibos em `.toolbelt/` ou no diretório do git | Dentro de caminhos que workers ou tickets podem gravar; `.toolbelt/` é caminho sensível |

## Segurança

| Ameaça | Mitigação |
|--------|-----------|
| O conteúdo de um ticket guia o agente (prompt injection) para uma resolução maliciosa | Sem ferramentas, workdir vazio e somente leitura, splice (M15); dados delimitados por nonce (M16); validação (M17); `novel_lines` relatado (M19); nenhuma ref se move sem um recibo revisado (M13) |
| Um agente edita arquivos além do conflito | Ele não grava nada; o tb faz splice só dos IDs de região que enviou (M15, M17) |
| Caminhos sensíveis (hooks, CI, `.toolbelt/`, caminhos do guard) resolvidos por regra ou agente | Nunca resolvidos pelo `semantic` nem pelo `prompt`, mesmo com `--allow-sensitive` (M14); o gate de caminhos sensíveis existente continua rodando ao aplicar |
| Segredos em prompts ou recibos | Caminhos da lista de negação e arquivos com match de segredo nunca são enviados (M14); uma resposta que acrescenta um segredo é recusada (M17); recibos não guardam stderr nem ambiente (M20) |
| Recibo forjado ou editado | Guardado fora do repositório e do diretório do git (0700/0600); o ID é o seu sha256; a aplicação checa heads dos tickets, hashes de entrada e a árvore resultante (M21) |
| Recibo velho aplicado depois que o alvo andou | `start_sha` precisa conferir (M21); o avanço por CAS continua valendo |
| Sandbox ausente ou quebrado | `NewRunner` falha fechado (M25); falhas de isolamento abortam o merge, sem nova tentativa (M18) |
| A config do repositório escolhe o agente ou aumenta orçamentos | `merge.*` e as chaves do agente vêm só da config de usuário (M23, M24) |
| Execução de código pelo git durante a resolução | Args de git endurecidos em tudo; o `git merge-file` não lê atributos nem roda drivers |
| Exaustão de recursos | Tetos de região, arquivo, prompt e stdout; timeouts por arquivo e total; teto de quantidade de arquivos (tabela do M23) |
| Rede | O tb não abre nenhuma; só o filho do agente configurado pelo humano pode (invariante) |

## Critérios de aceite

### Fase A (`semantic`)

1. `fail` e `last_wins` produzem texto e JSON idênticos byte a byte aos da release anterior à Fase A (testes golden nas fixtures existentes).
2. Dois tickets que editam linhas adjacentes, sem sobreposição, do mesmo arquivo: `fail` conflita; `semantic` aplica as duas edições, `files[].status` é `semantic` e uma entrada de `resolutions` tem `rule: disjoint`.
3. Dois tickets que adicionam imports diferentes ao mesmo bloco de import Go: `semantic` resolve com `go_imports`, o arquivo faz parse, e os imports ficam ordenados e sem duplicatas.
4. Dois tickets que mudam a mesma linha de jeitos diferentes: `semantic` sai com `ErrConflict`, motivo `overlap` com as faixas de linhas, e nenhuma ref muda.
5. Conflitos binários, de symlink, gitlink, delete/modify, add/add e de modo nunca são resolvidos e trazem seu motivo.
6. Um conflito em caminho sensível nunca é resolvido pelo `semantic`, com ou sem `--allow-sensitive`.
7. Uma região cuja resolução deixaria uma linha de marcador ausente das entradas, ou um arquivo Go que deixa de fazer parse, fica sem resolução (`parse_failed`).
8. Um arquivo cujo conteúdo tem linhas como `=======` ou `<<<<<<<` (um título Markdown, uma fixture de teste) é lido corretamente graças ao `--marker-size`.
9. Uma chave `merge.semantic.*` na config do repositório é ignorada com `config_untrusted`.
10. MCP `tb_merge` com `strategy: semantic` é dry-run salvo `apply:true` e devolve o mesmo JSON da CLI.

### Fase B (`prompt`)

11. Uma proposta nunca avança uma ref, com ou sem `--dry-run`, e imprime o ID do recibo, o caminho e o `result_tree`.
12. Com um agente falso (`vendor: custom`, `tools_disabled: true`) que devolve regiões válidas, a proposta grava um recibo; a aplicação com `--resolutions` não inicia nenhum processo de agente (checado com um falso que falha se rodar) e produz uma árvore igual a `result_tree`.
13. Cada uma destas respostas falsas deixa o arquivo sem resolução com o motivo indicado: nonce errado, ID de região faltando ou sobrando (`agent_protocol`); texto grande demais (`agent_oversized`); linha de marcador nova (`agent_markers`); arquivo Go que deixa de fazer parse (`parse_failed`); segredo acrescentado (`agent_secret`); `refuse` (`agent_refused`).
14. Um arquivo da lista de negação, ou cujas regiões ou contexto têm um segredo, nunca aparece em nenhum prompt (um agente falso que grava a entrada mostra isso), e fica sem resolução com `secret_path` ou `secret_in_context`.
15. Uma aplicação falha sem mover refs quando: os bytes do recibo mudaram (`receipt_tampered`); o alvo andou ou o head de um ticket mudou (`receipt_stale`); as entradas de uma região diferem, ou a árvore difere (`receipt_mismatch`); o recibo tem regiões sem resolução (`receipt_incomplete`).
16. Num host sem Landlock, o `prompt` falha com `no_landlock` antes de integrar; nada cai para o `semantic`.
17. Um agente que usa ferramentas ou deixa um processo escapado aborta o merge inteiro sem recibo e sem nova tentativa; um timeout tem uma nova tentativa.
18. `egm.agent_cmd` ou `merge.prompt.*` na config do repositório são ignorados com `config_untrusted`; sem `egm.agent_cmd` na config de usuário, o `prompt` falha com `agent_unavailable`.
19. MCP `tb_merge` com `strategy: prompt` falha com `prompt_cli_only`.
20. O recibo não guarda stderr, nenhum valor de ambiente e nenhum valor de `env.pass` (checado procurando um canário).

## Fases

### Fase A: `semantic` (um PR)

- `internal/merge/strategy.go`: constantes e parsing de estratégia (M1).
- `internal/merge/semantic.go`: classificação dos estágios, chamada ao `git merge-file`, escolha do marker-size, parser de regiões, regras `disjoint` e `go_imports`, pós-checagens (M3–M7).
- `internal/merge/merge.go`: o `pick` encaminha conflitos ao resolvedor semantic; campos de `Conflict`, `File` e `Result` (M9, M10).
- `internal/config`: `LoadUser` e as chaves `merge.semantic.*` (M23).
- `cmd/tb/merge.go`, `internal/registry/merge.go`: aceitar `semantic`; a tabela (M11, M27, M28).
- Testes: os critérios da Fase A, com fixtures montadas por `git` em repositórios temporários, como em `merge_test.go`.

### Fase B: `prompt`

- `internal/merge/prompt.go`: template do prompt, uso do runner, validação do payload, linhas novas (M12–M19).
- `internal/merge/receipt.go`: gravação do recibo, limpeza, checagens da aplicação (M20–M22).
- `internal/config`: `merge.prompt.*`.
- CLI `--resolutions`; MCP `prompt_cli_only`.
- Se importar `internal/egm` a partir de `internal/merge` criar um ciclo ou puxar coisa demais, extrair antes o runner para `internal/agentrun`, com os testes do egm inalterados.

### Fase C (opcional, depois de uso)

- `--verify`: rodar o build/lint do `tb verify` (sem testes ocultos) no worktree temporário antes de avançar. Ele roda comandos do repositório fora do sandbox, então é opt-in. Ver DECISÃO DO USUÁRIO 6.
- Mais regras determinísticas (regiões só de acréscimo, outras sintaxes de import), cada uma com seus critérios.
- `prompt` pelo MCP só como proposta, se a DECISÃO DO USUÁRIO 3 permitir.

## Docs a atualizar com a implementação

- **README:** o item do roadmap e a linha do `tb merge` na tabela de funcionalidades.
- **`docs/cli.md`:** a seção do `tb merge` (tabela de estratégias, flags, recibos, motivos novos).
- **`docs/arquitetura.md`:** o comentário do exemplo "Merge results (strategies: …)".
- **`cmd/tb/merge.go`**: texto de ajuda e a descrição do MCP `tb_merge`.
- **CHANGELOG**, e o espelho pt-BR de cada um.

## DECISÕES DO USUÁRIO

Elas mudam o modelo de confiança; a spec escolhe um padrão para cada uma.

1. **`prompt` em duas etapas** (padrão) versus uma chamada com `--accept-agent`. Duas etapas custam um comando a mais, mas mantêm um humano entre o agente e as refs.
2. **`semantic` aplica numa chamada** (padrão). O `disjoint` resolve conflitos que o git recusa de propósito. A alternativa é tornar o `semantic` só de proposta, como o `prompt`.
3. **`prompt` pelo MCP**: recusado na Fase B (padrão). Alternativa: só proposta, nunca aplicação.
4. **Linhas novas**: só relatar (padrão) versus um `max_novel_lines: 0` padrão, que limita o agente a escolher e ordenar linhas existentes.
5. **Reusar a config de agente do egm** (padrão) versus um `merge.agent_cmd` separado.
6. **`--verify` antes de avançar** (Fase C, opt-in, build sem sandbox) versus nunca rodar comandos do repositório a partir do merge.

## Questões em aberto

- O `last_wins` deveria ganhar um modo por hunk (manter o lado mais recente só nas regiões em conflito), já que o parser de regiões vai existir?
- Uma regra `append_both` (os dois lados inserem no fim do mesmo bloco; mantém ours, depois theirs) é segura o bastante para o `semantic` quando o arquivo Go continua fazendo parse?
- O `prompt` deveria enviar o mandato ou o texto de aceite do ticket como contexto? Ajuda a resolver, mas é mais texto não confiável no prompt.
- O recibo deveria carregar uma atestação (por exemplo, assinada pela chave de assinatura git do usuário) para times que compartilham recibos?
