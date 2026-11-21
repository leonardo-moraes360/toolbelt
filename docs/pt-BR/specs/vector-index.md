[🇺🇸 English](../../specs/vector-index.md) | 🇧🇷 Português

# Índice vetorial opcional para o `tb graph search` — spec

Status: **V0, V1 e V2 implementadas**; deixou de ser experimental: recall@10 em 44 consultas por intenção sobre este repositório (all-MiniLM-L6-v2 Q8_0): lexical 0,273, semantic 0,670, hybrid 0,568, então hybrid − lexical = +0,295 ≥ +0,10 ([dogfood](../campo/dogfood.md) #45). O modo semantic teve a maior nota nesta suíte; o hybrid mantém os acertos por nome exato. O shim ainda embeda um texto por chamada nativa: o lote multi-sequência (`n_seq_max ≥ 64`) ainda está pendente. Registro de decisão: [ADR-018](../adr/018-cgo-componentes-nativos.md) (cgo, llama.cpp vendorizado, gate de qualidade C/C++).

O índice vetorial acrescenta busca semântica ao grafo de código:

- **Os embeddings** vêm de um modelo GGUF, executado pelo llama.cpp vendorizado num processo filho em sandbox, sem acesso à rede.
- **O armazenamento** é um arquivo ao lado do `graph.json`.
- **A busca** funde o ranking BM25 com a similaridade vetorial.

O recurso é opt-in. Desligado, não muda nenhuma saída atual.

Ele sai em três fases, cada uma usável sozinha:

- **V0:** release com cgo e o gate nativo. Todo alvo é compilado com cgo, com o llama.cpp vendorizado, o shim em C e o job de CI `native`. A única mudança visível é a linha do motor no `tb version`.
- **V1:** embeddings e busca pela CLI.
  - o filho `tb __graph-embed` e o bloqueio de rede do sandbox;
  - o pacote compartilhado `internal/secrets`;
  - o armazenamento de vetores, `tb graph index --vectors` e `tb graph search --mode`.
- **V2:** MCP e evidência.
  - os parâmetros `mode` e `vectors` das tools MCP;
  - um filho mantido aquecido no `tb mcp`;
  - a avaliação que decide se o recurso deixa o status experimental.

## Problema

- **A busca lexical não capta a intenção.** O `tb graph search` ranqueia títulos, caminhos e pacotes por BM25. Uma consulta como "onde recusamos fontes com symlink" não encontra nada a menos que compartilhe palavras com um nome (`lstatChain`), e o agente acaba voltando ao grep.
- **Não há opção semântica local.** O ADR-003 e o ADR-006 deixaram os embeddings para a "Fase 2", e o único design registrado supunha o sqlite-vec. Nenhum dos dois combina com um índice em `graph.json` nem com a postura offline e de release verificada do projeto.
- **Valor não comprovado.** O estudo de campo ([SINTESE.md](../campo/SINTESE.md)) não observou essa falha. O recurso tem de medir seu ganho antes de o item do roadmap ser fechado.

## Objetivos

- G1: busca semântica e híbrida sobre o código e os docs que o grafo indexa, offline, sem rede e sem serviço externo.
- G2: nenhuma mudança para quem não fizer opt-in. O `tb graph search` sem `--mode`, o JSON dele e os padrões das tools MCP continuam idênticos byte a byte.
- G3: o código nativo não alcança o repositório, nem o HOME, nem a rede. A inferência roda num filho em sandbox, e o modelo é fixado por hash.
- G4: uma avaliação neste repositório que informa o recall@10 dos modos lexical e híbrido.

## Não-objetivos

- Backends de GPU, builds de CPU acima do piso do ADR-018, download de modelo pelo tb, APIs remotas de embedding.
- Vetores por nó (por função): os nós do grafo não têm intervalos de linhas hoje, então o V1 gera embeddings de trechos de arquivo (ver Unidades).
- Usar vetores em `callers`, `impact`, `explain` ou `path`, ou como fontes do EGM.
- Substituir o BM25: lexical continua sendo o modo padrão.

## Configuração

O `tb graph` lê a config pelo `config.Load()`, que mescla o `.toolbelt/config.yaml`. As chaves novas não podem vir do repositório, porque escolhem o que o código nativo carrega. O V1 acrescenta uma leitura só de usuário:

- **`config.LoadUser`:** o `LoadUserConfig` do egm, generalizado para qualquer seção. Lê o arquivo de `-c`, ou a config global, e nunca o arquivo do repositório.
- **Filtro do repositório:** o `graph` só carrega `graph.vector` e `graph.embed` pelo `LoadUser`.
- **Aviso:** um `.toolbelt/config.yaml` de repositório que defina qualquer uma delas recebe um aviso `config_untrusted` (o mesmo código do egm, R34), e o valor é ignorado.
- **MCP:** o `tb mcp` usa o mesmo caminho.

| Chave | Padrão | Significado |
|-----|---------|---------|
| `graph.vector.enabled` | `false` | Liga o recurso; sem ela, `--vectors` e `--mode semantic|hybrid` falham com `vectors_disabled` |
| `graph.embed.model` | — | Caminho absoluto para um modelo de embedding GGUF |
| `graph.embed.model_sha256` | — | Obrigatória; se não bater, o carregamento é recusado (`model_mismatch`) |
| `graph.embed.query_prefix`, `doc_prefix` | `""` | Prefixos de tarefa que alguns modelos exigem (ex.: `search_query: `) |
| `graph.embed.threads` | `min(4, NumCPU)` | Threads de inferência |
| `graph.embed.timeout` | `2m` | Por lote; um timeout mata o filho (`embed_timeout`) |
| `graph.embed.allow_unsandboxed` | `false` | Sem o sandbox completo (sem Landlock ou seccomp no Linux; darwin; windows), o filho recusa com `embed_unsandboxed`, a menos que seja true |
| `graph.vector.max_chunks` | `50000` | Acima disso, a indexação é recusada (`vectors_too_many`) |
| `graph.vector.chunk_lines`, `chunk_bytes`, `overlap_lines` | `40`, `1536`, `8` | Formato dos trechos: no máximo `chunk_lines` linhas e `chunk_bytes` bytes |

O arquivo do modelo é checado pelo pai, e de novo pelo filho antes de carregar:
- o caminho é absoluto;
- todo componente é alcançado por `Lstat`, sem symlink (no macOS, `/tmp` e `/var` são symlinks: use o caminho `/private/...`);
- todo diretório pai pertence ao usuário ou ao root e nenhum outro usuário pode escrever nele (exceto diretórios sticky como `/tmp`; escrita de grupo só para o grupo primário do próprio usuário) (unix);
- o arquivo é aberto sem seguir symlink, é regular e pertence ao usuário atual;
- está fora do repositório (V1: só o `tb graph` conhece o repositório);
- o sha256 dele é igual a `model_sha256`;
- depois do load nativo, o caminho ainda aponta para o arquivo do hash (mesmo dispositivo, inode, tamanho e mtime), senão o load é recusado (`model_mismatch`).

## Unidades

Uma **unidade** é um trecho de um arquivo que o indexador do grafo já lê (suas linguagens e docs, dentro de `graph.max_file_bytes`).

- **Pacote compartilhado de segredos:** o `internal/egm` importa o `internal/graph`, então o graph não pode importar o egm. O V1 move o `Redact` e os globs de negação da R7 (`denyBaseGlob`, `denyPathGlob`) para um pacote folha, `internal/secrets`, ao lado de `mem.SecretPatterns`. O egm e o graph passam a importá-lo, e o egm mantém o comportamento (os testes dele não podem mudar).
- **Arquivos pulados:** um arquivo cujo nome bate com a lista de negação nunca é lido para embedding.
- **Redação primeiro:** o **arquivo inteiro** é redigido antes de ser dividido em trechos, então um segredo que atravessa a fronteira entre trechos (um bloco PEM) ainda é pego. Um segredo só chega ao filho como `[REDACTED]`.
- **Divisão em trechos:** os trechos são cortados em fronteiras de linha, com no máximo `chunk_lines` linhas e `chunk_bytes` bytes, e `overlap_lines` de sobreposição. Uma única linha acima de `chunk_bytes` é cortada numa fronteira UTF-8.
- **Identidade:** cada unidade registra o caminho, o intervalo de linhas, o sha256 do texto redigido e uma flag `truncated`. A flag é marcada quando o filho precisou cortar a unidade no contexto do modelo.

Um acerto semântico é informado como o nó do arquivo, com o intervalo de linhas do melhor trecho em `lines`. Nós de docs (vault e Markdown) são mapeados do mesmo jeito.

## O filho: `tb __graph-embed`

O pai inicia o filho reexecutando o próprio binário (`os.Executable()`) como `tb __graph-embed`, com o caminho do modelo e o hash esperado como argumentos.

### Sandbox

O sandbox ganha o `DenyNetwork`, um filtro seccomp instalado pelo trampolim: `socket(2)` e `socketpair(2)` falham com `EPERM`. Onde o Landlock ABI ≥ 6 estiver disponível, ele também restringe o escopo de sockets unix abstratos e de sinais.

O filho roda com:
- `DenyNetwork`;
- leitura só dos diretórios que o sandbox sempre concede (`/usr`, `/lib*`, `/etc`, `/proc`, `/sys`, …), mais o arquivo do modelo (leitura) e o binário `tb` (leitura e execução);
- um **diretório temporário privado vazio** como workdir só leitura;
- um tmp privado, um HOME vazio, `MinimalDev` e o ambiente mínimo.

Por isso ele não consegue ler nem o repositório nem o HOME, e não tem rede.

Fallbacks:
- **Linux sem Landlock ou seccomp, darwin e windows:** o filho recusa (`embed_unsandboxed`), a menos que `allow_unsandboxed` esteja definido. Nesse caso ele roda como processo separado, com o ambiente mínimo, e todo comando que o usou imprime um aviso `embed_unsandboxed`.
- **Build sem a tag `embednative`:** o filho responde `embed_unavailable`.

### Configurações do motor no shim

- Um modelo cujo tipo de pooling é `NONE` é recusado (`model_unsupported`): ele não gera vetor de sentença.
- Para modelos não causais, `n_ubatch ≥ n_ctx` e `n_seq_max ≥ 64`, para que uma requisição seja um único lote de sequências (V1, com o protocolo; o shim da V0 gera o embedding de um texto por chamada como sequência 0, com `n_ctx = min(n_ctx_train, 8192)`).
- Um texto com mais de `n_ctx` tokens mantém os primeiros `n_ctx − 1` tokens e o último (o token especial final).
- Os vetores são lidos por sequência e normalizados em L2.

### Protocolo

JSON lines em stdin e stdout:

1. **Handshake:** depois de verificar o hash e carregar, o filho escreve `{"v":1,"engine":"llama.cpp","commit":"…","model_sha256":"…","dims":N,"n_ctx":M}`.
2. **Requisições:** cada requisição é `{"id":k,"kind":"doc"|"query","texts":["…"]}`, com no máximo 64 textos e 1 MiB.
3. **Respostas:** cada resposta é `{"id":k,"vectors":["<base64 of dims little-endian float32>",…],"truncated":[i,…]}` ou `{"id":k,"error":"…"}`. `truncated` lista os índices dos textos que foram cortados em `n_ctx` tokens.

Limites, todos aplicados pelo pai:
- **Tamanho da resposta:** cada linha de resposta é limitada a `64 × ceil(dims × 4 × 4/3) + 64 KiB`. Não há limite pela vida inteira do filho, porque o filho aquecido atende a muitas requisições.
- **Erros de protocolo:** uma linha maior ou malformada, um `id` errado ou um vetor com o comprimento errado matam o filho e falham com `embed_protocol`.
- **Quedas:** uma queda ou um sinal falha com `embed_crashed` e o status de saída.

O pai acrescenta os prefixos (`query_prefix` / `doc_prefix`).

## Armazenamento de vetores

O armazenamento é `.toolbelt/graph.vec`, ao lado do `graph.json`, com modo 0600, escrito de forma atômica (arquivo temporário mais rename).

- **Layout:**
  - o magic `TBVEC1`;
  - um cabeçalho JSON prefixado pelo tamanho: `v`, `engine_commit`, `model_sha256`, `dims`, o formato dos trechos, `prefixes_sha256`, `graph_fingerprint` (o `Fingerprint` do índice `graph.json` contra o qual foi gerado) e `count`;
  - `count` registros com caminho, intervalo de linhas, sha256 do texto, `truncated` e `dims` float32 little-endian.
- **Leitura:** o cabeçalho tem de ser interpretável, `count` tem de ser no máximo `max_chunks` e o tamanho do arquivo tem de bater exatamente. Caso contrário o armazenamento é `vectors_invalid`, e o tb o reconstrói em vez de ler além do fim.
- **Atualidade:** o armazenamento fica **desatualizado** quando qualquer um destes difere da config atual: `model_sha256`, `dims`, o formato dos trechos ou os prefixos. Um armazenamento desatualizado nunca é consultado (`vectors_stale`).
- **Incremental:** o `--vectors` só refaz o embedding das unidades cujo sha256 do texto ainda não está guardado com o mesmo modelo, e descarta as unidades de arquivos que sumiram.

## CLI

```bash
tb graph index --vectors                 # indexa, depois cria ou atualiza o graph.vec
tb graph search "<query>" --mode hybrid  # lexical (padrão) | semantic | hybrid
```

- **Modos:**
  - **lexical:** o ranking de hoje, sem mudança.
  - **semantic:** similaridade de cosseno do vetor da consulta contra todas as unidades (força bruta), agrupada por nó de arquivo (melhor trecho), top `--limit`. Um acerto cujo nó de arquivo não está mais no índice é descartado.
  - **hybrid:** Reciprocal Rank Fusion com `k = 60` sobre o ranking lexical e o ranking semântico. Empates são desfeitos pelo ID do nó.
- **JSON:** em `semantic` e `hybrid`, cada acerto ganha `lines`, `score_semantic` e, no hybrid, `rank_lexical`, `rank_semantic` e `score_fused`. O JSON lexical não muda.
- **Vetores desatualizados:** quando o `graph_fingerprint` difere do índice atual, `semantic` e `hybrid` ainda respondem, com `"stale_vectors": true` e um aviso. O `--mode` nunca reindexa.

## MCP (V2)

- O `tb_graph_search` ganha `mode` (`lexical` por padrão).
- O `tb_graph_index` ganha `vectors` (booleano, `false` por padrão).
- O `tb mcp` mantém um filho aquecido por servidor, morto depois de 5 minutos ocioso e na saída, para que uma consulta não pague o carregamento do modelo a cada vez.
- Os erros são mapeados para os mesmos motivos.

## Avaliação (V2)

`internal/graph/eval/semantic.yaml` guarda pelo menos 25 consultas sobre este repositório. Cada consulta lista os caminhos esperados, escrita como intenção ("onde fontes com symlink são recusadas"), não como nomes.

Um teste condicionado a `TB_EMBED_MODEL` (pulado sem ela) informa o recall@10 dos modos lexical, semantic e hybrid. O resultado, com o modelo e o commit do motor, é registrado no [dogfood.md](../campo/dogfood.md).

O recurso só deixa o status experimental (item do roadmap no README fechado) se o recall@10 do modo hybrid for pelo menos o do lexical mais 0,10. Caso contrário, os docs continuam marcando-o como experimental e o item fica aberto.

## Modelo de smoke do CI

O V0 fixa um modelo de embedding GGUF pequeno para o CI:
- no máximo 50 MB;
- uma licença permissiva;
- um tipo de pooling diferente de `NONE`.

O candidato é o all-MiniLM-L6-v2 quantizado em Q8_0 (Apache-2.0). A URL e o sha256 dele ficam em `.github/embed-smoke-model.txt`. O CI o baixa com o hash conferido e o guarda em cache; ele nunca é commitado. No darwin e no windows, o smoke test roda com uma config de teste que define `allow_unsandboxed: true`.

## Resumo de segurança

| Ameaça | Mitigação |
|--------|------------|
| Arquivo de modelo hostil (bug no parser GGUF) | Caminho e sha256 só da config de usuário; o arquivo pertence ao usuário e fica fora do repositório; o hash é conferido de novo no filho; é interpretado no filho em sandbox, sem rede |
| Bug de memória no llama.cpp ou no shim | Gate nativo (ADR-018): build com sanitizers, libFuzzer, análise estática; filho fora do processo em tempo de execução |
| Inicializadores estáticos em todo comando | O piso de CPU evita SIGILL; a checagem com strace mostra que a inicialização estática não faz I/O |
| Segredos no texto do embedding | Lista de negação por nome; `Redact` do arquivo inteiro antes da divisão em trechos; o `graph.vec` é 0600 e local (`.toolbelt/` é ignorado) |
| Um repositório escolhendo o que é carregado | Toda chave nova é lida pelo `config.LoadUser`; valores do repositório geram aviso e são ignorados |
| Esgotamento de recursos | `max_chunks`, o tamanho da requisição, o limite da resposta e o timeout por lote |

## Fases

### V0: release com cgo e gate nativo

Estes arquivos mudam juntos:
- `third_party/llama.cpp/` (o subconjunto em allowlist), seu `MANIFEST` e `ADVISORIES.md`, e `scripts/vendor-llama.sh`;
- `internal/embed/native/` (o shim e seus bindings cgo, build tag `embednative`);
- `.goreleaser.yaml` (`CGO_ENABLED=1`, `-tags embednative`, `CC`/`CXX` zig, `hooks.pre` por build com CMake, `CGO_LDFLAGS` por alvo, flags de hardening);
- `.github/workflows/ci.yml` (o job `native`; a regra de `import "C"` é um teste, `TestOnlyNativeImportsC`), `release.yml` (a matriz de smoke tests, QEMU, `rockylinux:8`), `codeql.yml` (`c-cpp` com o passo de checagem do SARIF; o job `go` continua sem tag);
- `.github/embed-smoke-model.txt`.

O `tb version` imprime `embed: llama.cpp <tag> (<commit>)`.

### V1: embeddings e busca pela CLI

Estes arquivos mudam juntos:
- `internal/secrets/` (movido do egm);
- `internal/sandbox` (`DenyNetwork`);
- `internal/embed/` (o filho, seu protocolo e o cliente do pai);
- `internal/graph/vector.go` (unidades, armazenamento, busca, RRF);
- `cmd/tb/graph.go` (`--vectors`, `--mode`);
- `internal/config` (`LoadUser` e as chaves).

### V2: MCP e avaliação

Estes arquivos mudam juntos:
- `internal/mcp` (os parâmetros e o filho aquecido);
- `internal/graph/eval/`.

O relatório da avaliação vai para `docs/campo/dogfood.md`.

## Docs a atualizar com a implementação

- **README:** o item do roadmap e a linha do `tb graph`.
- **`docs/cli.md`:** `graph index` e `graph search`.
- **`docs/release.md`:** artefatos nativos, os pisos de glibc e de CPU, a matriz de smoke tests.
- **ADRs:**
  - ADR-018: status aceito depois do V0.
  - ADR-001, ADR-003 e ADR-014: as notas de status já apontam para cá.
- **CHANGELOG**, e o espelho pt-BR de cada um.

## Critérios de aceite

### V0

1. Cada um dos seis alvos compila com cgo, pelo zig ou pelo fallback nomeado para aquele alvo. O resultado do spike (a toolchain por alvo e o tamanho do binário comparado ao da v0.0.10) é registrado no PR do V0 antes de o ADR-018 ser aceito.
2. A matriz de smoke tests nativos roda `tb version` e um smoke test de embed nos seis alvos, cada um no próprio runner, mais x86-64-v2 sob QEMU. Uma falha bloqueia a release.
3. Os artefatos Linux cumprem tudo isto:
   - rodam em `rockylinux:8`;
   - o `objdump -T` não mostra nenhuma versão de símbolo acima de `GLIBC_2.28`;
   - o `checksec`/`readelf` mostram PIE, RELRO completo, NX, canaries e chamadas fortificadas.
4. O `tb version` nomeia o motor, a tag e o commit. O SBOM lista o llama.cpp e o ggml com o commit e o hash do subconjunto do manifesto.
5. O job `native` falha com cada um destes:
   - um warning do compilador no shim;
   - um achado do `clang-tidy` ou do `cppcheck`;
   - um diff de formatação;
   - um alerta do CodeQL em `internal/embed/native/`;
   - uma árvore vendorizada que não bate com o `MANIFEST`;
   - um relatório do ASan ou do UBSan;
   - uma queda no libFuzzer;
   - um `import "C"` fora de `internal/embed/native`;
   - um strace do `tb version` diferente do build sem tag.
6. O job de advisories falha com um advisory que afeta a tag fixada (correção acima do pin, ou sem correção). Uma exceção em `ADVISORIES.md` para um componente que não é compilado vale por 30 dias.
7. Um `go build ./...` e um `go test ./...` sem tag continuam funcionando, e o `tb __graph-embed` responde `embed_unavailable`.

### V1

8. Com `graph.vector.enabled` desligado, a saída e o JSON do `tb graph search` são idênticos byte a byte aos da release anterior ao V1, e `--vectors`/`--mode semantic` falham com `vectors_disabled`.
9. Nenhum destes carrega:
   - um modelo cujo sha256 difere de `model_sha256` (`model_mismatch`);
   - um caminho de modelo com symlink ou dentro do repositório (`model_path`);
   - um modelo com pooling `NONE` (`model_unsupported`).

   Uma config de repositório que defina uma chave de `graph.vector`/`graph.embed` é ignorada com `config_untrusted`.
10. Um arquivo da lista de negação nunca é lido para embedding. Um segredo, inclusive um que atravessa a fronteira entre trechos, só chega ao filho como `[REDACTED]` (checado por um filho falso que registra sua entrada). Os testes de redação do egm passam sem mudança depois da mudança para `internal/secrets`.
11. Uma segunda execução de `--vectors` sem mudanças de arquivo não envia texto nenhum ao filho. Editar um arquivo refaz o embedding só dos trechos alterados desse arquivo.
12. Mudar o hash do modelo, os prefixos ou o formato dos trechos deixa o armazenamento desatualizado: ele nunca é consultado (`vectors_stale`), e o próximo `--vectors` o reconstrói.
13. Estas falhas do filho deixam o `graph.vec` inalterado:
    - uma linha de resposta malformada ou grande demais, ou um vetor com o comprimento errado (`embed_protocol`);
    - uma queda (`embed_crashed`);
    - um timeout (`embed_timeout`).
14. No Linux com Landlock e seccomp, um filho falso hostil não consegue fazer nada disto:
    - ler um arquivo do repositório ou do HOME;
    - escrever fora do seu tmp privado;
    - criar um socket de qualquer família.

    Sem eles, e no darwin e no windows, o filho recusa com `embed_unsandboxed`, a menos que `allow_unsandboxed` esteja definido.
15. O `hybrid` funde por RRF (`k = 60`) e é determinístico para entradas iguais. Um acerto cujo nó sumiu é descartado. Um `graph.vec` truncado ou corrompido é `vectors_invalid`, nunca uma queda.

### V2

16. O `tb_graph_search` sem `mode` e o `tb_graph_index` sem `vectors` se comportam como antes. Com eles, batem com a CLI, e o filho aquecido é reutilizado entre consultas e morto depois de 5 minutos ocioso.
17. O teste de avaliação informa o recall@10 dos três modos, e o resultado é registrado com o modelo e o commit do motor. A marca de experimental só é removida quando hybrid ≥ lexical + 0,10.
