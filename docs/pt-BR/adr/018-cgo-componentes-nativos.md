[🇺🇸 English](../../adr/018-cgo-componentes-nativos.md) | 🇧🇷 Português

# ADR-018: cgo para componentes nativos, começando pelo llama.cpp vendorizado, sob um gate de qualidade C/C++

## Status

proposto (2026-11-21). Só passa a aceito depois que o spike do V0 da [spec do índice vetorial](../specs/vector-index.md) compilar e passar no smoke test em todos os alvos (critérios 1 a 3 da spec).

Ele emenda:
- o [ADR-001](001-go-vs-rust.md): o binário estático único;
- o [ADR-014](014-graphify-reimplementacao.md): o cgo adiado.

O primeiro usuário é o índice vetorial.

## Contexto

O roadmap prevê um índice vetorial opcional para o `tb graph search` e inferência local opcional. Os dois precisam de um modelo de embedding que rode na máquina do usuário.

- **Não há opção em Go puro.** Nenhum runtime mantido em Go puro roda os modelos de embedding atuais (GGUF ou ONNX) com velocidade utilizável. O modelo tem de vir de C ou C++, ou de um processo fora do `tb`.
- **O build de hoje.** A release é `CGO_ENABLED=0` para seis alvos: linux, darwin e windows em amd64 e arm64. O ADR-001 conta o binário estático único como requisito. O ADR-014 adiou o tree-sitter porque o cgo complica a release.
- **Postura.** O projeto é fail-closed:
  - um sandbox Landlock (ADR-004);
  - git endurecido (R4c);
  - uma release verificada (sha256, cosign keyless, SBOM, SLSA).

  Código nativo não pode ser o ponto fraco dessa cadeia.
- **O que o sandbox não cobre.** O sandbox confina só o sistema de arquivos. Ele não isola a rede (sem regras de rede do Landlock, sem seccomp, sem netns) e sempre concede leitura dos diretórios do sistema (`/usr`, `/etc`, `/proc`, `/sys`, …).
- **Evidência.** O estudo de campo ([SINTESE.md](../campo/SINTESE.md)) não observou nenhuma falha que vetores resolveriam. O recurso tem de continuar opt-in e medir seu ganho.

Duas escolhas foram feitas ao escrever este ADR:

1. **Motor:** o [llama.cpp](https://github.com/ggml-org/llama.cpp), vendorizado numa tag fixada e compilado do código-fonte. É MIT, C/C++, não tem dependências externas e roda modelos de embedding GGUF. O onnxruntime foi rejeitado porque é distribuído como binário pré-compilado que nenhum gate consegue analisar.
2. **Escopo:** a **release inteira** é compilada com cgo. Não existe um segundo artefato em Go puro.

## Decisão

1. **cgo em todos os alvos da release.**
   - **Opções de build:** a release passa `-tags embednative` e compila só para CPU. Sem Accelerate, BLAS, Metal, CUDA ou Vulkan; sem `GGML_BACKEND_DL` nem `GGML_CPU_ALL_VARIANTS`, que exigem carregamento dinâmico.
   - **Piso de CPU, explícito:**
     - amd64 mira **x86-64-v2** (SSE4.2, sem AVX), com todas as opções `GGML_AVX*`, `GGML_FMA` e `GGML_F16C` desligadas;
     - arm64 mira **armv8.0-a** com NEON.

     Os inicializadores estáticos do llama.cpp rodam em todo processo `tb`. Uma instrução acima do piso derrubaria o `tb` inteiro com SIGILL, não só o embedding. O embedding fica mais lento do que num build nativo, e esse é o preço aceito.
   - **Builds de desenvolvimento:** um build sem a tag (`go build ./...`, `go test ./...`) não tem código nativo. O `tb __graph-embed` dele responde `embed_unavailable`.
2. **Vendorização.**
   - O llama.cpp e o ggml são vendorizados em `third_party/llama.cpp/` como um **subconjunto em allowlist** de uma única tag upstream. Ele contém o núcleo do `ggml` e o backend de CPU, `src/`, `include/`, os arquivos CMake de que eles precisam e as licenças; nada de examples, tools, server, rpc, gguf-py ou testes.
   - `scripts/vendor-llama.sh <tag>` gera o subconjunto de forma reproduzível a partir do tarball upstream.
   - O `MANIFEST` registra a tag, o commit, o sha256 do tarball, o hash exato do subconjunto e a licença.
   - Não há patches locais. Um upgrade é um único PR que roda o script e muda o manifesto.
3. **Código nativo em tempo de execução.**
   - **Linkagem:** a linkagem estática coloca os inicializadores estáticos de C++ em todo processo `tb`, inclusive no trampolim do sandbox. Eles rodam antes do `main`.
   - **Fora do processo:** o código nativo só é *chamado* a partir de um filho oculto, `tb __graph-embed`, que fala JSON lines por stdin e stdout.
   - **Sandbox do filho:**
     - as regras do Landlock permitem leitura dos diretórios do sistema que o sandbox sempre concede, mais o arquivo do modelo e o binário `tb` (execução);
     - o workdir é um diretório temporário privado vazio, só leitura;
     - o HOME é vazio e o ambiente, mínimo;
     - os devices são mínimos;
     - **um filtro seccomp novo faz `socket(2)` e `socketpair(2)` falharem com `EPERM`**, e o escopo do Landlock ABI ≥ 6 é acrescentado onde estiver disponível.

     Assim, um bug de memória no llama.cpp, ou um arquivo de modelo hostil, só consegue corromper esse filho. Ele não alcança o repositório nem o HOME, e não tem rede.
   - **Plataformas sem o sandbox:** no Linux sem Landlock ou seccomp, e no darwin ou windows, o filho recusa (`embed_unsandboxed`), a menos que a config de usuário defina `graph.embed.allow_unsandboxed: true`.
   - **Regra de import:** uma regra do depguard só deixa `internal/embed/native` importar `"C"`.
4. **Um gate de qualidade C/C++ moldado pelo uso.** É um job novo de CI, `native`, obrigatório para o merge. O código C/C++ é usado de três jeitos, e cada um tem suas checagens.

   | Código | Dono | Checagens |
   |------|-------|--------|
   | O shim (`internal/embed/native/*.c`, `*.h`), o único código que chama o llama.cpp | este repo | `-Wall -Wextra -Wpedantic -Werror`; `clang-tidy` (bugprone, cert, clang-analyzer, performance) com zero achados; `cppcheck --enable=warning,portability --error-exitcode=1`; `clang-format --dry-run -Werror`; alertas do CodeQL `c-cpp` em `internal/embed/native/` falham o job por um passo de checagem do SARIF (os alertas sozinhos não bloqueiam) |
   | llama.cpp e ggml vendorizados | upstream | sem lint e sem patches; o `MANIFEST` tem de bater com o subconjunto; alertas do CodeQL em `third_party/` abrem uma issue e nunca bloqueiam nem são silenciados no lugar; um job agendado de advisories (abaixo) |
   | Tudo o que é compilado junto | ambos | flags de hardening por plataforma (abaixo), checadas nos artefatos; um build com sanitizers do shim e do vendor; um harness libFuzzer |

   - **Advisories:** um job agendado lê os [advisories do GitHub](https://github.com/ggml-org/llama.cpp/security/advisories) e o OSV para o llama.cpp. Um advisory afeta o pin quando a tag com a correção está acima da tag fixada, ou quando não há correção. Um advisory que afeta o pin falha o job e bloqueia releases. A exceção é um advisory num componente que não é vendorizado ou não é compilado (server, rpc, um backend de GPU, …): ele pode receber uma exceção de 30 dias em `third_party/llama.cpp/ADVISORIES.md`, com o motivo, revisada num PR. Uma exceção vencida volta a bloquear.
   - **Hardening:**
     - todos os alvos: `-O2 -fstack-protector-strong`; em amd64 também `-fcf-protection`;
     - linux: `-D_FORTIFY_SOURCE=2` (o nível 3 exige glibc ≥ 2.34, acima do piso), `go build -buildmode=pie` e `-extldflags "-Wl,-z,relro,-z,now"`;
     - darwin: PIE é o padrão;
     - windows: `-buildmode=pie` com `--dynamicbase --nxcompat` (ASLR de alta entropia). O `-fstack-protector-strong` só é usado onde a libssp linka.

     O job checa os artefatos linux com `readelf`/`checksec`: PIE, RELRO completo, NX, stack canaries e chamadas fortificadas presentes. O `objdump -T` não pode mostrar nenhuma versão de símbolo acima de `GLIBC_2.28`.
   - **Sanitizers:**
     - As libs estáticas vendorizadas também são compiladas com `-fsanitize=address,undefined`.
     - Os testes de embed rodam com `go test -asan -tags embednative` (UBSan por `CGO_CFLAGS`/`CGO_LDFLAGS`) em linux/amd64 e linux/arm64.
   - **Fuzzing:** um harness libFuzzer exercita os pontos de entrada do shim (texto → tokenização → embedding, e formatos de lote) com o modelo de smoke do CI sob ASan: 5 minutos por PR e 1 hora toda noite. O fuzzing do parsing de GGUF continua sendo trabalho do upstream, porque o modelo é fixado pelo usuário.
   - **Inicialização estática:** o `tb version` roda sob `strace -f -e trace=file,network`. O trace tem de ser igual ao de um build sem a tag, para que os inicializadores estáticos não façam I/O.
5. **Builds e smoke tests.**
   - **Spike do V0:** antes da aceitação, o V0 compila cada alvo com um [zig](https://ziglang.org) fixado como `CC`/`CXX`, com o `hooks.pre` por build do goreleaser (CMake) e `CGO_LDFLAGS` por alvo, e linka a libc++ estaticamente.
   - **Fallbacks, nomeados de antemão:**
     - darwin: clang nativo em runners macOS, com split/merge do goreleaser;
     - windows: llvm-mingw.
   - **Piso de glibc:** os alvos Linux miram a glibc 2.28, e um container `rockylinux:8` confere isso.
   - **Matriz de smoke tests nativos:** todo alvo roda `tb version` e um smoke test de embed no próprio runner (ubuntu-24.04 amd64 e arm64, macos-15 arm64, macos-15-intel, windows-2025 amd64 e windows-11-arm), mais x86-64-v2 sob `qemu-x86_64 -cpu Nehalem`. Uma falha bloqueia a release.
6. **A cadeia da release cobre as partes nativas.**
   - O SBOM ganha um componente CycloneDX para o llama.cpp e o ggml (tag, commit, licença, hash do subconjunto).
   - O `tb version` imprime o motor e o commit dele.
   - Os checksums, a assinatura cosign e a proveniência SLSA não mudam.
7. **O modelo é dado do usuário.**
   - O caminho e o sha256 vêm só da config de usuário.
   - O arquivo tem de ser regular, do usuário, alcançado sem nenhum symlink e fora do repositório.
   - O filho confere o hash de novo antes de carregar.
   - O tb nunca baixa um modelo.

## Consequências

### Positivas

- Embeddings locais sem serviço externo, sem rede e sem Python. O mesmo motor pode servir depois à inferência local.
- O risco nativo fica contido duas vezes: pelo gate antes do merge e pelo filho em sandbox, sem rede, em tempo de execução.
- O sandbox ganha um bloqueio de rede por seccomp que outros filhos podem usar.
- Uma toolchain cgo também libera o `go test -race` no CI.

### Negativas

- **Custo da release:** zig, CMake por alvo, uma matriz de smoke tests com seis runners, um job de QEMU e um build com sanitizers.
- **Deixa de ser um binário estático único:**
  - o Linux precisa de glibc ≥ 2.28;
  - o amd64 tem um piso de CPU (x86-64-v2);
  - o binário cresce, numa quantidade que o V0 tem de medir.
- **Inicializadores estáticos:** os inicializadores estáticos de C++ do llama.cpp rodam em todo comando do `tb`. Há uma checagem de que eles não fazem I/O, mas são uma superfície nova antes do `main`.
- **Ritmo do upstream:** o llama.cpp anda rápido. Um advisory que afeta o pin bloqueia releases até um upgrade, ou até uma exceção para um componente que não é compilado.
- **Base confiável:** um parser GGUF em C++ está no binário. O hash fixado e o filho em sandbox são mitigações, não uma garantia.

## Alternativas consideradas

- **Comando externo de embedding** (`graph.embed_cmd`), mantendo `CGO_ENABLED=0`. É o mais simples para a release. Foi rejeitado como caminho principal porque a qualidade e a configuração dependeriam do que cada usuário instala.
- **cgo num artefato opt-in separado** (`tb-embed`). Manteria o binário padrão estático, mas dividiria em dois a release e a documentação.
- **onnxruntime pré-compilado.** É um binário opaco: o gate só conseguiria fixar hashes.
- **sqlite-vec.** Não é necessário: o índice é o `graph.json`, e cosseno por força bruta em Go dá conta dos tamanhos esperados. Pode ser revisto por um build WASM, sem cgo.
- **Builds com AVX2/FMA para ganhar velocidade.** Rejeitado: os inicializadores estáticos rodam em todo comando, então uma CPU sem AVX2 perderia o `tb` inteiro, não só o embedding.
- **Componente em Rust.** Continua possível depois, atrás do mesmo protocolo do filho.
