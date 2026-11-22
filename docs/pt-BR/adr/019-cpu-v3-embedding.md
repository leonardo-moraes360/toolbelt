[🇺🇸 English](../../adr/019-cpu-v3-embedding.md) | 🇧🇷 Português

# ADR-019: kernels AVX2/FMA no filho de embedding, com guarda em tempo de execução

## Status

aceito (2026-11-21), depois que o spike (#165) passou. Emenda o [ADR-018](018-cgo-componentes-nativos.md) §1 (o piso de CPU no amd64) e a alternativa que ele rejeitou, "builds AVX2/FMA para velocidade".

## Contexto

O ADR-018 compila o llama.cpp para **x86-64-v2** (SSE4.2, sem AVX) no amd64. O motivo: os inicializadores estáticos do llama.cpp rodam em todo processo `tb`, então uma instrução acima do piso derrubaria o `tb` inteiro com SIGILL, não só o embedding. Ele aceitou um embedding mais lento como preço.

Esse preço se mostrou alto e o risco, menor do que se supunha. Os dois foram medidos em 2026-11-21 (all-MiniLM-L6-v2 Q8_0, llama.cpp b12600, 4 threads, um host amd64 local com AVX2):

| 64 textos | x86-64-v2 (release) | x86-64-v3 (AVX, AVX2, FMA, F16C, BMI2) |
|---|---|---|
| chunks de 1,5 KB | 8,6 s | 3,9 s (**2,2×**) |
| consultas curtas | 0,29 s | 0,15 s (1,9×) |

Agrupar os textos de uma requisição no shim (dogfood #47, incluído no #163) economizou só 4% na indexação: o custo é a aritmética sem AVX/FMA, não o número de chamadas. Um índice completo deste repositório no piso leva cerca de 25 minutos (dogfood #43).

**Os inicializadores estáticos não rodam AVX neste build.** Um build x86-64-v3 do `tb` rodou sob `qemu-x86_64 -cpu Nehalem`, que não tem AVX e no qual o QEMU dispara SIGILL em instruções AVX:
- `version`, `graph stats`, `graph search`, `mem list`, `sandbox check`, `bus channel list` e `guard check` saíram com 0, sem instrução ilegal;
- `__graph-embed --smoke` morreu com SIGILL, como esperado, e o build x86-64-v2 passou no mesmo smoke.

Ou seja, o código AVX só roda quando o motor é chamado, e isso só acontece no filho de embedding. O risco do ADR-018 é real para o filho e, até aqui, não para os outros comandos. É evidência de um build e de um conjunto de comandos, não uma prova: uma mudança no compilador ou no upstream poderia pôr código AVX num caminho que todo comando executa. O gate abaixo tem de pegar isso.

## Decisão

1. **Kernels de CPU do amd64 no nível AVX2.** O release compila o backend de CPU do ggml vendorizado com `GGML_AVX`, `GGML_AVX2`, `GGML_FMA`, `GGML_F16C` e `GGML_BMI2` ligados. É a parte de cálculo do x86-64-v3, não o nível inteiro (sem LZCNT nem MOVBE); o requisito para o usuário é dito como x86-64-v3 por cautela. `GGML_AVX512*`, `GGML_AVX_VNNI` e `GGML_AMX_*` continuam desligados, porque muitas CPUs atuais não os têm. O código Go mantém o `goamd64: v1` do release, e o shim, o llama e o resto do ggml mantêm o alvo base atual (o `-mcpu=baseline` do zig); só as flags do ggml-cpu (`ARCH_FLAGS`) mudam.
2. **Uma guarda em tempo de execução no filho.** No início do `tb __graph-embed` (antes de `Serve` e de `Smoke`, ou seja, antes do primeiro `native.Open`), o filho verifica, em Go (`golang.org/x/sys/cpu`), `X86.HasAVX`, `HasAVX2`, `HasFMA`, `HasF16C` e `HasBMI2`. `HasAVX`/`HasAVX2` já incluem a checagem de OSXSAVE/XCR0 de que o SO salva o estado AVX, e o BMI2 é verificado porque hipervisores podem escondê-lo mesmo expondo AVX2. Se algum faltar, o filho recusa com um motivo novo, **`embed_cpu_unsupported`**, e o código nativo nunca roda. O pai o trata como os outros motivos do filho (ele entra nos códigos conhecidos que o cliente lê do stderr do filho). Nenhum outro comando é afetado.
3. **O gate mantém o código AVX fora de todos os outros caminhos.** O que protege os outros comandos é a invariante de que só o filho chama código nativo; as checagens abaixo a garantem.
   - **Só o filho chama o nativo:** o `TestOnlyNativeImportsC` já limita `import "C"` ao `internal/embed/native`; um teste novo também exige que o `native.Open` só seja alcançado a partir do comando `__graph-embed` (uma checagem do grafo de chamadas sobre os pacotes).
   - **Onde o código AVX pode ficar:** um script desmonta o binário inteiro e reprova se alguma instrução VEX ou EVEX estiver fora de símbolos definidos pelos objetos do ggml-cpu. Isso pega o dobramento de COMDAT: uma função inline ou de template (de headers da libc++, por exemplo) instanciada numa unidade de tradução do ggml-cpu compilada com `-mavx2` pode ser a cópia que o linker mantém para todos os chamadores, inclusive os inicializadores estáticos do llama. A visibilidade de símbolos não impede esse dobramento num link estático, então quem verifica é a auditoria, não uma flag de compilador.
   - **Sob QEMU:** um job de CI roda o binário de release sob `qemu-x86_64 -cpu Nehalem` para o `tb version` e alguns comandos somente leitura (`graph stats`, `mem list`, `sandbox check`). A inicialização estática é a mesma para todo comando, então isso é um smoke, não a proteção principal.
   - **Checagem da guarda:** sob a mesma CPU do QEMU, `__graph-embed --smoke` tem de falhar com `embed_cpu_unsupported`, não com SIGILL.
   - **Hardware real:** a matriz de smoke mantém uma execução real de embedding em runners x86-64-v3. O job x86-64-v2 sob QEMU do ADR-018 passa a esperar `embed_cpu_unsupported` do smoke de embedding, e que o `tb version` passe.
4. **O arm64 não muda** (armv8.0-a com NEON). Kernels com dotprod ou i8mm são medidos antes de qualquer mudança, pelas mesmas regras.
5. **Docs.** O `docs/release.md` e o README dizem que embeddings exigem uma CPU x86-64-v3 no amd64 (Intel Haswell de 2013, AMD Excavator de 2015, ou mais novas), e que todos os outros comandos mantêm o piso x86-64-v2.

### Resultado do spike (#165)

O spike pedia três coisas: os seis alvos compilando com os kernels v3 e o gate rodando no CI, medições num runner de CI, e o recall batendo com o da v2 com margem de 0,01 (a aritmética muda com FMA, então os vetores não são idênticos bit a bit). Tudo passou:

- **Builds e smoke:** os seis alvos compilaram. O smoke real de embedding passou nos seis runners, inclusive macos-15-intel e windows-2025 amd64, que têm AVX2.
- **Gate:**
  - `scripts/avx-audit.py`: ok, com 31.327 instruções de nível AVX, todas em símbolos que só a libggml-cpu.a define. Um build de controle com `-mavx2 -mfma` em todas as libs nativas reprova (2.736 símbolos, por exemplo `quantize_row_*_ref` na libggml-base).
  - QEMU Nehalem: os comandos passaram, e o smoke de embedding recusou com `embed_cpu_unsupported`.
  - O `TestOnlyChildReachesNative` passou.
- **Runner de CI (ubuntu-24.04, 4 threads):**

  | | v2 | v3 |
  |---|---|---|
  | 64 chunks + 64 consultas | 36,3 s | 12,2 s (**3,0×**) |
  | avaliação de recall, 2.721 chunks | 18m52s | 5m49s (**3,2×**) |
  | recall@10 lexical / semântico / híbrido | 0,273 / 0,670 / 0,568 | idêntico |
  | tamanho do binário | 27,23 MB | 27,57 MB (+1,2%) |

## Consequências

### Positivas

- Embedding 2–3× mais rápido no amd64, onde está a maioria dos usuários (3,2× num runner de CI), com recall idêntico e binário 1,2% maior.
- O piso do ADR-018 vira um fato verificado para todo comando, e não uma suposição sobre os inicializadores estáticos.

### Negativas

- Em CPUs amd64 sem AVX2, o embedding para de funcionar (`embed_cpu_unsupported`): modelos anteriores a 2013 e alguns mais novos de entrada (Pentium, Celeron e Atom sem AVX). Todos os outros comandos continuam funcionando nelas.
- A guarda, a auditoria por desmontagem e o smoke sob QEMU são custo novo de CI.
- A evidência se apoia em testes, não numa propriedade do build: uma mudança no upstream poderia levar código AVX para um caminho que roda em todo lugar. O teste do grafo de chamadas e a auditoria por desmontagem existem para pegar isso antes de um release.

## Alternativas consideradas

- **Manter x86-64-v2 (o ADR-018 como está).** É o mais simples e funciona em todo lugar, com metade da velocidade. Rejeitado pelas medições; o `TB_X86_KERNELS=v2` ainda o compila (benchmarks, plano B).
- **Duas variantes de kernel num binário só.** Compilar o ggml-cpu duas vezes (v2 e v3) com símbolos prefixados e escolher uma por CPUID no filho. CPUs antigas continuam com embedding, mas o build vendorizado ganha uma etapa de renomear símbolos e o dobro de código de kernel. Adiado até que um usuário com CPU anterior à v3 precise de embeddings.
- **`GGML_BACKEND_DL` com `GGML_CPU_ALL_VARIANTS`.** É o dispatch do próprio upstream, mas ele carrega `libggml-cpu-*.so` em tempo de execução. Isso quebra o arquivo único, e cada caminho de carga precisaria de verificação (o ADR-018 rejeitou carga dinâmica).
- **Um artefato `tb-embed` separado em v3.** Manteria o binário principal em v2 sem dúvida nenhuma, mas o ADR-018 rejeitou dividir o release e as docs.
