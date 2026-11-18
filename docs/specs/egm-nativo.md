🇧🇷 Português | [🇺🇸 English](../en/specs/egm-nativo.md)

# Spec: EGM nativo, Fase A, núcleo (`tb egm review|loop|resolve|status`)

Autor: toolbelt · Status: Rascunho (reestruturado; rodada 3 da série nova) · Data: 2026-11-17 · Decisão: [ADR-014](../adr/014-egm-nativo.md) · Complemento: [egm trust](egm-trust.md)

## Visão geral

O `tb egm` revisa **documentos de design** (specs e ADRs) com goldfish (revisores em contexto fresco) e repete o loop de revisão até convergir.

Esta spec é o **núcleo** da Fase A:
- isolamento dos revisores;
- revisão, loop, resolve e árbitro;
- defesas contra **conteúdo de terceiros** (repositório clonado, arquivo baixado, documento revisado).

As proteções contra fraude ativa do próprio autor, que roda como o mesmo usuário, ficam na spec [egm trust](egm-trust.md): cadeia de hash, âncora, pins, confirmação por TTY e gate por ref de confiança.

## Modelo de ameaças assumido

- **Conteúdo hostil (coberto):** o repositório, o arquivo baixado, o `.git/` dele e o documento revisado podem ser hostis. O `tb egm` não pode executar nada desse conteúdo nem vazar segredo do usuário para o provedor.
- **Elephant honesto, mas desleixado (coberto):** quem escreve o documento pode tentar atalhos acidentais ou preguiçosos, como marcar como corrigido sem corrigir, contestar até passar ou rebaixar severidade. O loop não deixa esses atalhos virarem `PASS`.
- **Fraude ativa do mesmo usuário (fora do escopo):** um processo com shell como o mesmo usuário pode editar a config ou o estado, trocar o binário do agente ou forjar entradas. Isso fica para a spec [egm trust](egm-trust.md), e o núcleo não promete nada contra esse caso.

## Glossário

| Termo | Definição |
|-------|-----------|
| elephant | Quem escreveu o documento e chama o `tb egm`: o agente principal ou um humano. |
| goldfish | Revisor one-shot em processo isolado. Vê só o próprio prompt. |
| revisor | Vaga do preset (R10), por exemplo `style-feature`. Cada revisor usa uma persona; uma persona pode servir a vários revisores. |
| revisor dono | O revisor responsável por uma linhagem e o único que a reavalia (R19, R20). |
| árbitro | Goldfish que julga uma contestação (R29). |
| persona | Lente do revisor: o que procurar e como julgar (`personas/<nome>.md`). |
| standard | Critério de qualidade (`standards/<nome>.md`). Os estruturais listam headings exigidos (R13). |
| tipo de documento | `spec` ou `adr` (R6). Define o standard estrutural; o plano detalhado define os revisores obrigatórios. |
| documento sob gate | Documento que casa com `egm.doc_globs` (R33). |
| heading | Linha ATX (`#` a `######` seguidos de espaço) fora de bloco de código cercado. As comparações usam a linha inteira, com os `#`, sem espaços nas pontas. |
| preset `consolidated` | Rodada com os revisores obrigatórios (R10) em paralelo, sem checkpoints. |
| config de usuário | Seção `egm:` de `config.GlobalConfigPath()`. |
| `<state>` | `$XDG_STATE_HOME/tb` ou, sem essa variável, `~/.local/state/tb`. No macOS, `os.UserConfigDir()/toolbelt/state`. |
| diretório de estado (`<dir-estado>`) | `<state>/egm/<repo-id>/<slug>/`. Fica fora de qualquer worktree e é compartilhado pelos worktrees do mesmo repositório (R21). |
| pacote | Diretório só-leitura dentro do diretório de estado, com o que foi entregue a um goldfish. É também o workdir dele (R1). |
| nonce | Valor aleatório de 128 bits por execução. Delimita os blocos de dado e precisa ser ecoado na saída. Prova que a saída é desta execução, **não** que o modelo resistiu a injeção. |
| atestação | Lista `checked` de trechos literais do documento que o revisor leu, conferida pelo tb (R17). |
| fontes citadas da linhagem | O documento mais a união dos `evidence[].path` **verificados** das ocorrências que passaram pelo `resolveSource` na rodada (`cited`, R21). |
| `origin_round` / `opened_round` | Rodada em que a linhagem nasceu / última rodada em que ela ficou `open` ou `reopened` (uma reabertura atualiza `opened_round`). |
| linhagem bloqueante | Linhagem blocker ou major em `open`, `reopened`, `contest_pending`, `escalated` ou `fixed` sem confirmação. |
| política | `policy_sha`: o sha256 do JSON canônico com revisores obrigatórios, motor, `vendor`, `tools_check`, `clean_rounds`, `match`, `attest`, `detail_headings`, `doc_types`, `refs_ignore`, `max_source_bytes` e `max_packet_bytes` efetivos (R21). |
| conteúdo | `content`: o sha256 da lista ordenada por path de pares `(path, sha256 dos bytes)` do documento e de **todas** as fontes enviadas a qualquer goldfish da rodada, mais o `checks_sha` (o sha256 das entradas dos checkers: cada path verificado com tipo e existência). Os bytes vêm do worktree no modo interativo e do blob no modo gate (R7a). |
| registro de convergência | Registro que liga um `content` a uma convergência (R23). |
| rodadas contadas | As `egm.clean_rounds` rodadas limpas consecutivas, sobre o mesmo `content`, que geraram um registro de convergência. |
| ciclo | Rodadas desde a última convergência, ou desde o `review`/`--restart`. Teto e orçamento contam por ciclo (R23, R27). |
| slug | Identificador do documento (R14). |
| FeatureRange / DetailedRange | Linhas antes do heading de plano detalhado e a partir dele (R6). Sem esse heading, FeatureRange é o documento inteiro e DetailedRange fica vazio. |
| linhagem `F-<k>` | ID estável de um finding entre rodadas (`k` sequencial por slug). |
| ocorrência `<reviewer>-r<round>-<i>` | ID de um finding numa rodada. |
| status de revisor | `completed` (saída válida), `not_run` (falhou, R16, ou foi interrompido, R3/R27), `unavailable` (persona LLM sem agente) ou `skipped` (sem range aplicável, R6/R17). |
| perfil de vendor | Receita por CLI: argv, flags sem ferramentas, autenticação, envelope de saída e detecção de uso de ferramenta (R4, R5, R15). |

## Problema

O `tb review` revisa diffs com heurísticas. Não revisa documentos, não tem revisor LLM e não converge. O EGM upstream faz isso em prosa e deixa isolamento, estado e aprovação a cargo do harness e do modelo. Ver o [ADR-014](../adr/014-egm-nativo.md#contexto).

## Objetivos

- G1: isolar cada revisor por processo, sandbox, HOME descartável, env mínimo e ausência de ferramentas, e verificar essa ausência em execução.
- G2: produzir um finding estruturado (`egm.Finding`) que `tb stack` e o MCP consigam consumir.
- G3: deixar a convergência a cargo do tb, persistida e ligada a um conteúdo exato. Um atalho desleixado (seção anterior) não chega a `PASS`.
- G4: rodar o `tb egm` sobre um repositório de terceiros ou um arquivo baixado sem executar binário e sem expor segredo escolhido por esse conteúdo, seja via config, symlink ou config de git.
- G5: ter um modo offline útil, com cobertura declarada, que não passa num gate estrito.
- G6: manter inalterados a CLI e o JSON de `tb review` e `tb drift`, e o comportamento de `tb merge` e do router.

## Não-objetivos

- O tb não pergunta nada ao usuário e não edita o documento revisado.
- O tb não altera a severidade dada por um revisor. Uma saída acima do teto da persona é inválida (R11).
- O tb não fala com a rede nem chama API de LLM.
- Gate em CI: o gate é **local** (R33a).
- Proteção contra fraude ativa do mesmo usuário: fica para [egm trust](egm-trust.md).
- Endurecer os loaders que o `tb stack` já roda antes de qualquer op: o `git diff base...head` e o `config.Load()` de `Prepare`, que hoje rodam sem as proteções do R4c e do R34. Num repositório não confiável, use `tb egm status --at <rev>` diretamente, que é endurecido.
- Fora desta fase:
  - presets `staged`/`feature`/`implementation`;
  - personas por path;
  - `intermediate-zoom-out`;
  - perfil `codex`;
  - override de agente por revisor;
  - verificação de símbolos no `tb graph`;
  - churn;
  - adaptador `review.Finding` → `egm.Finding`;
  - drift-report, bootstrap, draft e brainstorm.
- O texto das personas do upstream não é copiado.

## Jornadas de usuário (CUJs)

- **CUJ-1.** *Dado* `docs/specs/x.md` (versionado ou não) e o perfil `claude` configurado, *quando* rodo `tb egm review docs/specs/x.md`, *então* os revisores obrigatórios rodam isolados e recebo `round-1/report.{md,json}` com o veredito `IN_PROGRESS`. <!-- tb:egm-example -->
- **CUJ-2.** *Dado* que não há agente, *quando* rodo o mesmo comando, *então* os checkers determinísticos rodam, as personas LLM ficam `unavailable`, o tb explica por que não usou `llm` e o melhor veredito possível é `PASS_PARTIAL`.
- **CUJ-3.** *Dado* que a rodada 1 tem findings, *quando* edito as linhas apontadas (ou o código citado), rodo `tb egm resolve docs/specs/x.md --fix F-1,F-2` e depois `tb egm loop next docs/specs/x.md` duas vezes, *então* o revisor dono reavalia cada linhagem e as duas últimas rodadas, limpas e sobre o mesmo conteúdo, geram um registro de convergência. <!-- tb:egm-example -->
- **CUJ-4.** *Dado* o blocker `F-3`, que cita `internal/x.go`, *quando* rodo `tb egm resolve docs/specs/x.md --contest F-3 --evidence internal/x.go:40`, *então* F-3 fica `contest_pending` e o próximo `loop next` roda o árbitro primeiro. Uma segunda contestação de F-3 é recusada. <!-- tb:egm-example -->
- **CUJ-5.** *Dado* um repositório clonado cujo `.toolbelt/config.yaml` define `egm.agent_cmd` e `egm.env.pass`, *quando* rodo `tb egm review`, *então* esses valores são ignorados, o tb emite `config_untrusted` e nada do repositório é executado.
- **CUJ-6.** *Dado* um arquivo baixado com `.git/config` e `.gitattributes` hostis (filtros, fsmonitor, `core.worktree`), um `.git/index` forjado e um diretório versionado que é symlink para `~/.config`, *quando* rodo `tb egm review`, *então* nada é executado e nada fora do repositório é lido.
- **CUJ-7.** *Dado* que revisei uma spec num worktree de feature, fiz o merge e removi o worktree, *quando* rodo `status` em outro worktree com o mesmo conteúdo, *então* o veredito é o mesmo.
- **CUJ-8.** *Dado* que converjo na versão A e commito a versão B, *quando* rodo `tb stack egm`, *então* o gate avalia B (o `head`), não encontra registro e responde `STALE`.

## Plano técnico

```
elephant ──tb egm review|loop next|resolve|status──┐
                                                    ▼
┌─────────────────────────── tb ───────────────────────────┐
│ fase 1 (sem goldfish): config → raiz real do repo →      │
│   snapshot → fontes (caminho real) → redact → scan       │
│ fase 2: goldfish em sandbox.New(pacote RO, Tmp, Home,    │
│   env mínimo, Stdin, Stdout), sem tools; checkers        │
│ fase 3 (sem goldfish): validate (schema, teto, nonce,    │
│   atestação, tool-use, evidência) → match → linhagem     │
│   → state.json (atômico) → convergência → veredito       │
└──────────────────────────────────────────────────────────┘
```

O tb só chama git nas fases 1 e 3, nunca enquanto um goldfish roda (R3).

### Requisitos

**R0: spike.** Este requisito bloqueia R1, R2, R3, R4, R4b e R5, e a passagem do ADR-014 para `accepted`. R4c e R4d não dependem do spike.

Um protótipo em `bench/egm-spike/` deve provar que `claude -p`:
1. roda em `sandbox.New` com o pacote como workdir só-leitura, `TMPDIR` privado, HOME descartável, env mínimo (R4b) e `PATH` mínimo;
2. recebe o prompt por stdin e devolve JSON em stdout, separado do stderr;
3. roda **sem nenhuma ferramenta**, e o envelope permite detectar quando uma ferramenta é usada (R15a);
4. não carrega `~/.claude/CLAUDE.md`, as memórias nem os settings do usuário;
5. produz atestação (R17) válida;
6. não deixa processo-filho órfão sem `TB_EGM_OWNER` (R3);
7. não consegue ler `/proc/<ppid>/environ` do `tb` (a Landlock restringe ptrace a processos fora do domínio). O R1 é ajustado conforme o resultado.

O spike também deve:
- **Autenticação:** definir a ordem de preferência: (1) API key via `egm.env.pass`; (2) config dir do vendor no HOME descartável, por exemplo `CLAUDE_CONFIG_DIR`; (3) cópia de arquivo de credencial, só como último recurso.
- **Refresh de token:** testar se um refresh no HOME descartável invalida a credencial real.
- **Runtime:** definir os diretórios de runtime liberados (R4).
- **macOS:** registrar o Keychain como limitação.
- **Matching:** coletar ao menos 30 pares parafraseados e 30 pares distintos na mesma região, e calibrar R18–R20 para fusão indevida ≤ 5% e casamento perdido ≤ 20%.
- **Convergência:** medir a mediana de rodadas até 2 limpas em 2 ou 3 documentos reais. Se passar de 4, `max_rounds` e a regra de rodada limpa são revistos antes de implementar.

O resultado vai em `docs/campo/dogfood.md` (PT e EN).

**Sandbox e execução**

- R1: Em cada rodada, o tb grava `<dir-estado>/round-<n>/target.md`, o snapshot redigido do documento, e `round-<n>/sources/<path>`, o snapshot redigido de cada fonte enviada a algum goldfish (usado pelo `--fix`, R28). Para cada goldfish LLM, grava também `round-<n>/packets/<reviewer>/` com:
  - `prompt.txt`;
  - `sources.json`: path, `sha256` e as linhas incluídas;
  - `meta.json`: persona, standard, range, nonce e `argv_sha256`.

  O conteúdo vem do **worktree** no modo interativo e do commit avaliado no modo gate (R33). O workdir do goldfish é o pacote, fora do repositório.

  A Landlock libera para leitura o pacote, os paths do R4 e as regras de sistema que já existem em `addSystemReadRules`: `/bin`, `/sbin`, `/lib*`, `/usr/bin`, `/usr/sbin`, `/usr/lib*`, `/usr/libexec`, `/usr/share`, `/etc`, `/dev`, `/proc` e `/sys`. A leitura de `/proc/<pid>/environ` de outro processo passa pela checagem de ptrace, que a Landlock restringe para processos fora do domínio; o R0 confirma isso. A ausência de ferramentas (R5, verificada pelo R15a) é uma segunda barreira, não a única.
- R2: `sandbox.Config` ganha os campos abaixo. Com eles zerados, nada muda.
  - `ReadOnlyWorkDir bool`: a regra do workdir, hoje de leitura e escrita, passa a ser só-leitura.
  - `PrivateTmp string`: diretório gravável que substitui a regra de `os.TempDir()`, exportado como `TMPDIR`.
  - `Home string`: diretório gravável, exportado como `HOME`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME` e `XDG_DATA_HOME`.
  - `Path string`: exportado como `PATH`.
  - `Stdin io.Reader`.
  - `Stdout io.Writer` e `MaxStdoutBytes int`: o stdout vai só para esse writer. Se o limite estourar, a captura para e o `Result` traz `StdoutOverflow: true`. O stderr continua no `tailBuffer`.

  `Home` e `Path` sobrescrevem `Config.Env`. Com comando absoluto, `Run` deixa de chamar `exec.LookPath` e executa o path como foi passado. Ajuste em `addRule`: para arquivo regular, a máscara passa a ser `accessMask(readOnly, abi)` sem os direitos exclusivos de diretório (`ReadDir`, `Remove*`, `Make*`, `Refer`), como o `deviceFileMask` já faz.
- R3: o processo `tb egm` cria **um** `sandbox.Registry` com `NewRegistry("TB_EGM_OWNER")` e `SetOwnerVar("TB_EGM_OWNER")`, compartilhado por todos os goldfish e árbitros, e chama `sandbox.AcquireSubreaper()` uma vez. Sem subreaper (`ok=false`), o motor `llm` recusa com exit 2.
  - **Cada goldfish:** usa `sandbox.New` com `ProcessGroup: true` e esse Registry, `GroupOwner = egm:<slug>:r<n>:<reviewer>` (para o árbitro, `egm:<slug>:r<n>:arbiter:<F-k>`) e a mesma string em `TB_EGM_OWNER`.
  - **Git:** o tb não chama git enquanto há goldfish rodando. As chamadas de git (R4c) acontecem só nas fases 1 e 3.
  - **Sweep:** cada `Sweep()` atribui órfãos por `Orphan.Vars["TB_EGM_OWNER"]` e por `GroupOwner`.
    - Órfão de um goldfish da rodada, inclusive `ForeignGroup` com esse owner: esse goldfish vira `not_run` com `reason: escaped_process`.
    - A atribuição usa primeiro `Orphan.Vars["TB_EGM_OWNER"]`, que é o owner que o próprio processo declara, e depois `Orphan.GroupOwner`.
    - Órfão sem owner durante a fase 2: o tb mata os grupos de todos os goldfish ainda em execução, marca todos como `not_run` com `reason: unattributed_orphan`, e a rodada fica não limpa. A rodada não é abortada.
  - **Quando roda:** `Sweep()` roda depois que cada goldfish termina e ao fim da fase 2.
  - **Timeout:** `context.WithTimeout(egm.timeout_per_reviewer)`.
  - **Plataforma:** o motor `llm` exige Linux com Landlock.
- R4: para o perfil `claude`, o tb:
  1. cria, **por goldfish**, o HOME e o `PrivateTmp` em `os.TempDir()/tb-egm-<rand>/`, com permissão `0700` e um arquivo `owner.pid` com o pid do `tb`;
  2. autentica pela forma definida no R0;
  3. libera só-leitura o diretório do binário resolvido e os diretórios de `egm.home.read_only`;
  4. apaga tudo no final (com defer, inclusive em timeout) e varre os `tb-egm-*` com mais de 1h cujo `owner.pid` não está mais vivo.

  `egm.agent_cmd[0]` precisa ser absoluto. O processo roda com `PATH = egm.path` (padrão `/usr/bin:/bin`).
- R4b (env mínimo): o env do goldfish é montado do zero, sem `os.Environ()`, e contém só:
  - `HOME`, `TMPDIR`, `XDG_*` e `PATH` (R2);
  - `LANG=C.UTF-8`;
  - `TB_EGM_OWNER` e `TB_EGM_ATTEMPT` (`1` ou `2`);
  - as variáveis de `egm.env.pass`.

  O tb recusa com exit 2 nomes em `egm.env.pass` que comecem com `NODE_`, `PYTHON`, `LD_`, `DYLD_`, `RUBY`, `PERL5` ou `GIT_`, e também `BASH_ENV`, `ENV` e `PROMPT_COMMAND`.
- R4c (git do egm): o pacote `egm` tem as próprias opções e o próprio env de git em `internal/egm/git.go`. **Não** altera `internal/router` nem `internal/merge`.
  - **Opções:**
    - `-c core.fsmonitor=false`, `-c core.hooksPath=/dev/null`, `-c core.sshCommand=false`;
    - `-c diff.external=`, `-c core.pager=cat`, `-c core.untrackedCache=false`;
    - `-c submodule.recurse=false`, `-c diff.ignoreSubmodules=all`;
    - `-c core.excludesFile=/dev/null`, `-c core.attributesFile=/dev/null`;
    - `--no-ext-diff --no-textconv`, onde o subcomando aceita.
  - **Env:** parte de `os.Environ()` e remove tudo que começa com `GIT_`. Depois acrescenta `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_PAGER=cat`, `GIT_TERMINAL_PROMPT=0` e `GIT_DIR`/`GIT_WORK_TREE` explícitos, vindos do R4d. A exceção é o `diff --no-index`, que roda sem `GIT_DIR`/`GIT_WORK_TREE`, com cwd e `GIT_CEILING_DIRECTORIES` no diretório de estado.
  - **Timeout:** cada chamada tem um timeout fixo de 30 s. Se estourar, exit 2 com `reason: git_timeout`.
  - **Subcomandos permitidos (lista fechada):**

    | Subcomando | Uso |
    |------------|-----|
    | `ls-files -s -z -- <paths>` | Modo git e blob no índice, só para as fontes candidatas |
    | `ls-tree -z <rev> -- <paths>` | Modo git e blob num commit, só para as fontes candidatas (modo gate) |
    | `hash-object --no-filters -- <paths>` | Hash dos bytes do worktree, só para fontes já validadas pelo R7 |
    | `cat-file blob <rev>:<path>` | Conteúdo num commit (gate, árbitro) |
    | `rev-parse --verify`, `merge-base <a> <b>` | Refs e base de três pontos (R33) |
    | `diff --name-status --no-renames -z <a> <b>` | Diff de árvores entre commits (R33) |
    | `diff --no-index -U0` | Diff entre snapshots, fora de qualquer repositório (env acima) |
    | `check-ignore --no-index -- <paths>` | Exclusão de ignorados (R7) |

    Qualquer outra chamada de git no pacote é erro de programação, coberto por teste estático. `git status` não é usado, e o tb nunca faz uma varredura de todos os arquivos versionados.
- R4d (raiz real do repositório): o tb localiza o repositório subindo a partir do cwd até achar uma entrada `.git` no filesystem, sem consultar o git.
  - Se `.git` for diretório, `GIT_DIR` é esse diretório.
  - Se `.git` for arquivo `gitdir:` (worktree), o alvo precisa estar sob `<common>/worktrees/` de um repositório cujo `.git` é diretório. Se não estiver, exit 2.
  - `GIT_WORK_TREE` é sempre o diretório que contém `.git`. Um `core.worktree` em `.git/config` é ignorado, porque o env explícito tem precedência.
  - O realpath (`EvalSymlinks`) desse diretório é a **raiz real**.
  - **Antes de qualquer chamada de git**, o tb confere `.git` direto no filesystem:
    - um `.git` que é symlink é recusado;
    - num `gitdir:`, o arquivo `<gitdir>/gitdir` precisa apontar de volta para este `.git`, e `<gitdir>/commondir` precisa resolver para `<common>`;
    - os arquivos conferidos são `<common>/config` e `<gitdir>/config.worktree` (os dois, se existirem), `<gitdir>/HEAD`, `<gitdir>/index` (opcional: um `git init` novo não tem) e `<common>/objects/info/`; com `.git` diretório, `<common>` e `<gitdir>` são o próprio `.git`;
    - `config` e `config.worktree` precisam ser arquivos regulares de até 1 MiB, sem seções `[include]` nem `[includeIf …]`;
    - `HEAD` precisa ser arquivo regular de até 4 KiB, e `index`, se existir, arquivo regular sem limite de tamanho;
    - não pode existir `objects/info/alternates` nem `objects/info/http-alternates`.

    Qualquer violação dá exit 2 com `reason: hostile_git_dir`.
- R5: o argv do goldfish é `egm.agent_cmd` mais os flags do perfil que desligam **todas** as ferramentas; todo o conteúdo vai no prompt. O tb recusa com exit 2:
  - argv com `bypassPermissions`, `--dangerously-skip-permissions` ou algo de `egm.forbidden_flags`;
  - perfil `custom` sem `egm.tools_disabled: true`.
- R6 (ranges e tipo):
  - **Corte:** o tb divide o documento no **primeiro heading** cuja linha inteira seja igual a um item de `egm.detail_headings`.
  - **Tipo:** `egm.doc_types` mapeia globs para tipos. O padrão é `docs/adr/**` → `adr`; o resto é `spec`. O tipo define só o standard estrutural (R13).
  - **Revisores obrigatórios:**
    - com DetailedRange: os 9 do R10;
    - sem DetailedRange: os 6 sem DetailedRange.
  - **Cobertura:** numa `spec` sem DetailedRange, os 3 do detalhe ficam `skipped` (`reason: no_detailed_section`), o que é cobertura parcial (R24). Num `adr` sem DetailedRange, eles não são obrigatórios.

**Fontes, redação e prompt**

- R7 (fontes): o documento revisado e as fontes passam por estas regras.
  - **Documento revisado:**
    - é lido pelo realpath;
    - precisa ser arquivo regular dentro da raiz real (R4d); se não, exit 2;
    - pode não estar versionado (CUJ-1);
    - é sempre fonte;
    - se passar de `egm.max_packet_bytes`, exit 2.
  - **Candidatos a fonte:**
    - (a) links markdown relativos (`CollectMDLinks`), resolvidos em relação ao diretório do documento;
    - (b) spans entre crases simples (`CollectBacktickSpans`, função nova sobre `reBacktick`), resolvidos em relação à raiz.

    Um span só é candidato se:
    - não tiver espaço, `<`, `>`, `$`, `~`, `(`, `*`, `{`, `…` nem `:`;
    - terminar numa extensão de `rePathLike` (que ganha `json`, `yaml`, `yml` e `toml`) ou terminar em `/`.
  - **Resolução** (`egm.resolveSource`):
    1. remove fragmento e query;
    2. recusa path absoluto;
    3. junta com a base e aplica `path.Clean`;
    4. recusa se sair da raiz;
    5. aplica `Lstat` em **cada componente** a partir da raiz; qualquer componente que seja symlink exclui a fonte (`symlink`);
    6. exige que o último componente seja arquivo regular;
    7. confere `ls-files -s`: precisa estar no índice com modo `100644` ou `100755`.
  - **Exclusões, registradas em `skipped_sources` com o motivo:**
    - fora da raiz e URLs;
    - não versionados ou ignorados;
    - `.env*`, `*.pem`, `*.key`, `*.p12`, `id_*`, `credentials*`, `secrets*`, `*.tfvars`, `*.tfstate*`, `kubeconfig*`, `.npmrc`, `.netrc`, `.pypirc`, `service-account*.json`, `*.keystore` e `*.jks`;
    - qualquer fonte em que a redação (R8) encontre algum segredo (`secret_like`); o documento revisado é redigido, mas não excluído;
    - `.toolbelt/` e `.git/`;
    - `**/testdata/**`, `internal/egm/personas/`, `internal/egm/standards/` e `internal/egm/injection_patterns.txt`;
    - **outros documentos sob gate e arquivos sob `docs/en/`**, porque não são código e são revisados como documentos;
    - acima de `egm.max_source_bytes`;
    - o que exceder `egm.max_packet_bytes`.
  - **Candidato fora do índice:** um candidato que existe no worktree mas não está no índice gera o aviso `unstaged_source`.
  - **Efeitos:**
    - uma exclusão por `max_*` numa fonte de revisor com fontes de código torna a cobertura parcial;
    - só os revisores com fontes de código (R10) e o árbitro recebem fontes além do documento;
    - a mudança no `rePathLike` também altera as arestas de menção e de link (`EdgeLinks`) do `tb graph` para MD, TXT e HTML, e há teste de regressão para isso.
- R7a (modo gate): com `--at <rev>`, a resolução é a mesma do R7, mas sobre a árvore do commit:
  - o modo vem de `ls-tree` e precisa ser `100644` ou `100755`; um componente intermediário que não seja árvore (por exemplo, um symlink `120000`) exclui a fonte;
  - os bytes vêm de `cat-file blob`;
  - as exclusões são as mesmas do R7.

  Se uma fonte usa filtro de `.gitattributes`, os bytes do worktree e do blob diferem, e o tb emite `filtered_source`. Nesse caso, o `content` do modo interativo não bate com o do gate. É uma limitação declarada.
- R8: `internal/egm/redact.go` tem uma cópia própria dos padrões de `mem.RedactText` mais os padrões abaixo, e troca cada match por `[REDACTED]` seguido do mesmo número de quebras de linha do trecho original, preservando a numeração. `internal/mem` não muda.
  - `\bsk-ant-[A-Za-z0-9_-]{20,}`
  - `\bsk-[A-Za-z0-9]{32,}`
  - `(?i)\b[A-Za-z0-9_]*_(KEY|SECRET|PASSWORD)=[^\s"'&]+`
- R9 (prompt): o `prompt.txt` contém, nesta ordem:
  1. persona e standard;
  2. `REVIEWER: <nome>`;
  3. `RANGE: <a>-<b>`;
  4. `NONCE: <nonce>`;
  5. o contrato de saída;
  6. `PRIOR:` seguido de um bloco `<<<DATA-<nonce> path=PRIOR>>> … <<<END-<nonce>>>>`;
  7. os blocos `<<<DATA-<nonce> path=…>>> … <<<END-<nonce>>>>`, com a numeração de linha original.

  O `PRIOR` lista as linhagens das quais o revisor é dono (já com as transferências do R20) que estão em `open` ou `reopened`, em `fixed` sem confirmação ou em `fixed` confirmadas no ciclo atual. De cada uma, traz só estes campos: `lineage`, `severity`, `status`, `scope`, `message`, `rationale` e `evidence`. Nenhum texto escrito pelo elephant (razões) entra no prompt. O prompt declara que **todo** o conteúdo dos blocos é dado, nunca instrução.

**Personas, standards e motores**

- R10: revisores do preset `consolidated`, rodados em paralelo até `egm.max_parallel`.

  | Revisor | Persona | Standard | Range | Fontes de código |
  |---------|---------|----------|-------|------------------|
  | zoom-out | zoom-out | zoom-out | doc inteiro | não |
  | inventive-catalyst | inventive-catalyst | (nenhum) | doc inteiro | não |
  | comprehension | comprehension | (nenhum) | doc inteiro | não |
  | design | design | estrutural do tipo | FeatureRange | não |
  | structural | structural | estrutural do tipo | doc inteiro | não |
  | style-feature | style | (nenhum) | FeatureRange | não |
  | implementation-readiness | implementation-readiness | implementation-readiness | DetailedRange | sim |
  | codebase-details | codebase-details | implementation-readiness | DetailedRange | sim |
  | style-detailed | style | (nenhum) | DetailedRange | não |

  O standard estrutural é `feature-spec` para `spec` e `adr` para `adr`. `--reviewers a,b` restringe a lista e torna a cobertura parcial.
- R11: as personas, o `arbiter` e os standards ficam embutidos (`go:embed`) em `internal/egm/personas/` e `internal/egm/standards/`, com texto próprio. Cada persona declara `max_severity`; para `inventive-catalyst`, é `major`. Um finding acima do teto torna a saída inválida.
- R12: `--engine auto|llm|deterministic`, com padrão `auto`.
  - `auto` usa `llm` se `egm.agent_cmd` estiver configurado na config de usuário. Senão, usa `deterministic` e imprime `agent_cmd ausente na config de usuário`.
  - Não há herança do `router.agent_cmd`.
  - `--engine llm` sem agente é erro (exit 2).
- R13: os checkers abaixo e o `injection-scan` (R17a) rodam **nos dois motores** e não contam como revisores para cobertura.

  | Checker | Cobre parcialmente (motor `deterministic`) | Severidade | Verifica |
  |---------|--------------------------------------------|------------|----------|
  | `refs-exist` | codebase-details | `major` no DetailedRange; `nit` fora dele | Cada candidato (b) do R7 que contém `/` passa pelos passos 1 a 5 do `resolveSource`. Os que saem da raiz ou atravessam symlink são ignorados, sem dizer se existem. Os demais precisam existir, como arquivo ou diretório, antes das exclusões de versionamento e tamanho. Ignora: paths sob `.toolbelt/` ou `.git/`; paths `Create` do plano (após expandir `{a,b}`) e os diretórios que os contêm; o que casa com `egm.refs_ignore` (padrão `refs/**`); linhas com `<!-- tb:egm-example -->`. |
  | `required-sections` | structural | `major` | Headings exigidos pelo standard estrutural; basta uma das duas línguas |
  | `plan-files-listed` | implementation-readiness | `major` | O parsing descrito abaixo; só com DetailedRange |

  Headings exigidos pelo `required-sections`:

  | Standard | Headings exigidos |
  |----------|-------------------|
  | `feature-spec` | `## Visão geral` ou `## Overview`; `## Problema` ou `## Problem`; `## Objetivos` ou `## Goals`; `## Não-objetivos` ou `## Non-Goals`; `## Plano técnico` ou `## Technical Plan` |
  | `adr` | `## Status`; `## Contexto` ou `## Context`; `## Decisão` ou `## Decision`; `## Consequências` ou `## Consequences`; `## Alternativas Consideradas` ou `## Alternatives Considered` |

  Parsing do `plan-files-listed`:
  1. Usa a primeira tabela do DetailedRange que tenha uma coluna `Arquivo`/`File` e uma `Ação`/`Action`.
  2. Lê só a coluna `Arquivo`/`File`, e só os spans entre crases simples.
  3. O texto `(+ docs/en/)` fora de crases, numa célula, é uma diretiva: gera o espelho `docs/en/…` de cada path `docs/…` dessa célula.
  4. `{a,b}` expande para os dois paths. Com `Modify`, um `*` precisa casar com algum arquivo.
  5. A ação precisa ser `Create`, `Modify` ou `Delete`, e todo `Modify` ou `Delete` precisa existir.

  As linhagens de checker podem ser contestadas no árbitro. No motor `deterministic`, as personas LLM ficam `unavailable`, não há árbitro e `--contest` é recusado. Uma linhagem major do `injection-scan` nesse motor só sai por `--fix` ou por `--force-settle`, que é rebaixamento de linhagem. Isso é coerente com o melhor veredito offline ser `PASS_PARTIAL`.

**Contrato de saída e validação**

- R14: o slug é o path relativo em minúsculas, com `/` → `--` e `[^a-z0-9-]` → `-`, mais `-` e os 8 primeiros hex do sha256 do path. Um documento renomeado é um documento novo.
- R15: o stdout, sem o envelope do perfil, é validado por um validador escrito à mão em `internal/egm/validate.go`, sem dependência nova, que segue `internal/egm/schema/finding.schema.json`.
  - Envelope: `claude-json` usa o campo `result` e é o padrão do `claude`; `raw` é o padrão do `custom`.
  - Formato: `{"reviewer","nonce","checked":[{"heading","quote"}],"findings":[{"severity","scope":{"heading","lines":[a,b]},"message","rationale","evidence":[{"path","line"}]}],"prior":[{"lineage","still_holds":bool}]}`.
- R15a (uso de ferramenta): o perfil define como detectar uso de ferramenta no envelope.
  - No `claude-json`, conta como uso: `num_turns > 1`, entradas `tool_use` ou `permission_denials` não vazio.
  - Se houver uso detectado, o goldfish vira `not_run` com `reason: tools_used` e o relatório traz o aviso `tools_used`.
  - O `custom` declara `egm.tools_check: none`, e nesse caso o melhor veredito é `PASS_PARTIAL`.
- R16 (falhas): `not_run` nunca equivale a zero findings.
  - Com retry (uma tentativa a mais e depois `not_run` com `reason`): saída ausente, schema inválido ou teto excedido, nonce errado, atestação inválida, `prior` incompleto, `StdoutOverflow` e timeout.
  - Sem retry: processo escapado (R3), uso de ferramenta (R15a) e orçamento (R27).
- R16a: um finding recebe `unverified: true` quando o `evidence.path:line` não está nas fontes do pacote (o documento conta) ou quando o `scope.heading` não existe no snapshot.
  - Um finding `unverified` conta para o R23 como qualquer outro e pode ser contestado.
  - A evidência dele não entra em `cited`.
  - Ele aparece marcado no relatório.
- R17 (atestação): seja `H` o número de headings no range e `Q = egm.attest.min_quotes` (padrão 3).
  - **Mínimo:** cada revisor LLM devolve em `checked` ao menos `Q` entradas, cobrindo ao menos `min(H, max(1, ceil(H/3)))` headings distintos. Se houver texto citável, ao menos uma entrada não pode ser texto de heading.
  - **Validade de cada `quote`:** precisa aparecer **literalmente** no snapshot, sob o heading declarado e dentro do range, depois de normalizar espaços e remover `|`. Precisa ter de 20 a 200 caracteres, salvo quando é o texto de um heading.
  - **Casos especiais:**
    - range com menos de `Q × 20` caracteres de texto: o mínimo cai para 1;
    - range com texto e sem heading: zero headings exigidos e `heading` vazio;
    - range sem heading e sem texto: o revisor fica `skipped` (`reason: range_too_small`).
- R17a (scan anti-injeção): antes de cada rodada, o tb varre **todo** o snapshot (prosa, código e citações), o bloco `PRIOR` de cada revisor e as fontes enviadas.
  - **Padrões:** ficam em `internal/egm/injection_patterns.txt` (RE2, um por linha, sem diferenciar maiúsculas). O conteúdo inicial é **exatamente**:
    - `ignore (all |the )?(previous|prior|above) instructions`
    - `do not report`
    - `return (no|empty|zero) findings`
    - `you are (approved|pre-approved)`
    - `como revisor, (não|aprove)`
    - `output only (json|\{)`
    - `responda apenas (com|o) json`
  - **Finding:** os matches sob o mesmo heading, ou do mesmo arquivo de fonte, formam **um** finding `major` do `injection-scan`.
    - `scope` é a união das linhas.
    - Também traz `context` e `match_sha` (o sha256 do texto casado). `context` é `code` (bloco cercado ou código inline), `quote` (linha iniciada por `>`), `source` (fonte), `prior` (bloco `PRIOR`) ou `prose` (o resto).
    - Um match no `PRIOR` tira a linhagem de origem do `PRIOR` daquela rodada, passa essa linhagem para `escalated` (`by: rerun`) e gera o aviso `prior_injection`.
  - **Contestação** (R29):
    - as linhas de `scope` vão ao árbitro entre aspas e escapadas, num bloco `TRECHO-SINALIZADO`;
    - a contestação é recusada se houver termo dirigido ao árbitro (`arbiter`, `árbitro`, `withdraw`, `uphold`, `verdict`, `veredito`) a até 80 caracteres de algum match;
    - o scan roda sobre o excerto de evidência, isentando as linhas de `scope` das linhagens `injection-scan` em `open`, `reopened`, `contest_pending` ou `settled` com `match_sha` igual; outro match recusa a contestação.
  - **Reabertura:** uma linhagem `injection-scan` em `settled` **reabre** se o `match_sha` mudar.
  - **Goldens:**
    - esta spec gera uma única linhagem, cujo único match é o segundo item da lista de padrões acima;
    - o ADR-014 gera uma única linhagem, cujo único match é o segundo exemplo citado no seu §3;
    - nos dois casos, a contestação com o documento como evidência é aceita, e a retirada pelo árbitro leva a linhagem a `settled`.

**Matching, linhagem e estado**

- R18 (funções de comparação): `normalize(msg)` passa para minúsculas, tira acentos e pontuação, colapsa espaços e remove as stopwords de `internal/egm/stopwords.txt`.
  - `sim(a,b)` é o Jaccard dos tokens de `normalize`.
  - `overlap(r1,r2)` é `|r1∩r2| / min(|r1|,|r2|)`.
  - Os limiares ficam em `egm.match.sim` (padrão 0.3) e `egm.match.overlap` (padrão 0.5), calibrados no R0.
- R19 (fusão e dono): na mesma rodada, dois findings se fundem quando `sim ≥ egm.match.sim` e, além disso, `overlap ≥ egm.match.overlap` ou, sem `lines`, o mesmo `heading`. A fusão mantém a maior severidade e a lista `reviewers[]`.

  O dono é o revisor da ocorrência de **maior severidade**. Os empates se resolvem nesta ordem: revisor com fontes de código, depois a ordem do R10, e os checkers depois dos revisores.
- R20 (reavaliação): entre rodadas, quem reavalia é o **revisor dono**. As linhagens `open`, `reopened` e `fixed` desse revisor entram no `PRIOR` dele, e ele responde `prior[].still_holds`. Omitir uma linhagem invalida a saída.

  | Status | `still_holds: true` | `still_holds: false` |
  |--------|---------------------|----------------------|
  | open / reopened | continua | → fixed (`by: reviewer`, confirmado) |
  | fixed | → reopened | continua fixed (confirmado) |

  - **Linhagens de checker:** reavaliadas pela re-execução do checker.
    - Não reproduzida: vira `fixed`, confirmado.
    - Reproduzida: `fixed` vira `reopened`.
  - **Dono fora da rodada** (`skipped`, `unavailable` ou removido por `--reviewers`):
    - a linhagem passa para o próximo revisor, na ordem do R19, que esteja rodando e cujo range cubra o `scope` mapeado;
    - se não houver nenhum, o novo dono é o `zoom-out`, que tem o documento inteiro no range e é sempre obrigatório.
  - **Findings novos:** são casados com as linhagens existentes pelo mapeamento de linhas (`git diff --no-index -U0` entre os snapshots) e pelo R19.
    - Casou com uma linhagem `fixed`: ela reabre.
    - Casou com `settled`, `declined`, `contest_pending` ou `escalated` e a severidade nova **não é maior**: só registra a ocorrência, salvo o R17a.
    - Casou com uma dessas, mas a severidade nova é maior: vira uma nova `F-<k>` com `supersedes` apontando para a antiga.
    - Não casou: vira uma nova `F-<k>`.
- R21 (estado): o diretório de estado contém:
  - `state.json` (`0600`), gravado em arquivo temporário seguido de `rename` e `fsync` do diretório;
  - `round-<n>/`;
  - `lock`, um `flock` que o kernel libera se o processo morrer. Comandos que alteram o estado pegam `LOCK_EX|LOCK_NB` e, se o lock estiver ocupado, saem com exit 2 e `reason: locked`. O `status` lê sem lock, o que é seguro graças à escrita atômica.

  O `state.json` guarda:
  - **Rodadas:** `n`, `cycle`, `content`, `policy_sha`, `doc_sha256`, `head_commit`, `sources` (path → sha256), motor, revisores obrigatórios e executados com status, e `argv_sha256`.
  - **Mapa** de ocorrência para linhagem.
  - **Linhagens:** `severity`, `owner`, `persona`, `status ∈ open|fixed|reopened|contest_pending|settled|escalated|declined`, `confirmed`, `contests`, `origin_round`, `opened_round`, `cited`, `match_sha` (no `injection-scan`) e `downgrade` (R24).
  - **Transições:** comando, rodada, motivo e `by ∈ elephant|reviewer|rerun|arbiter`.
  - **Registros de convergência:** `content`, `policy_sha`, as rodadas contadas e as flags de rebaixamento de ciclo.

  O `repo-id` é o sha256 do realpath do diretório git comum (`<common>` do R4d). Worktrees do mesmo repositório compartilham o `repo-id`; clones e forks têm o seu.
- R22: uma linhagem `settled` ou `declined` que reaparece só registra a ocorrência, salvo o R17a e a severidade maior (R20).

**Convergência e veredito**

- R23 (rodada limpa e convergência): uma rodada é limpa quando, **após** o matching e a reavaliação, valem as três condições:
  - nenhuma linhagem blocker ou major está em `open`, `reopened`, `contest_pending` ou `escalated`;
  - nenhuma está `fixed` sem confirmação;
  - nenhum revisor obrigatório ficou `not_run`.

  Há convergência com `egm.clean_rounds` (padrão 2) rodadas limpas consecutivas **sobre o mesmo `content`**. Uma rodada limpa com `content` diferente da anterior reinicia a contagem em 1. A convergência grava um registro para o `content` e encerra o ciclo; a rodada seguinte abre um ciclo novo.

  Teto do ciclo: `capped` quando a rodada `egm.max_rounds` (padrão 5) do ciclo termina sem convergir. A partir daí, `loop next` sem `--past-cap` é recusado com exit 1 (`FAIL`). `loop next --past-cap` permite até `egm.max_past_cap_rounds` (padrão 3) rodadas extras e rebaixa o ciclo.
- R24 (rebaixamentos): há dois tipos, e todos ficam registrados.
  - **De linhagem**, valem enquanto a linhagem estiver no status: `forced` (`--force-settle`) e `declined` de `major`. Eles persistem entre ciclos. Saem só por `--fix` a partir de `settled` ou `declined` (R28), seguido da confirmação do dono (`still_holds: false`).
  - **De ciclo**, valem para o registro de convergência daquele ciclo:
    - `--past-cap` e `--restart`;
    - cobertura menor que a da primeira rodada do ciclo (por exemplo, `--reviewers` mais restrito), salvo `skipped` estrutural;
    - exclusão `max_*`;
    - revisor obrigatório `unavailable` ou `skipped`, ou spec do tipo `spec` sem DetailedRange (`no_detailed_section`);
    - perfil sem `tools_check`.
- R25 (veredito): o veredito é calculado sobre o **conteúdo-alvo**, que é o worktree no modo interativo ou o commit no modo gate (R33). As regras são avaliadas **nesta ordem**, e vale a primeira que casar:

  | # | Veredito | Condição |
  |---|----------|----------|
  | 1 | `NO_STATE` | Não há estado. |
  | 2 | `PASS` | Há registro de convergência para o conteúdo-alvo, com `policy_sha` igual ao atual, sem rebaixamento de ciclo, sem linhagem com rebaixamento de linhagem e sem nenhuma linhagem bloqueante. |
  | 3 | `PASS_PARTIAL` | Como a linha 2, mas com algum rebaixamento. |
  | 4 | `FAIL` | O ciclo atual está `capped` e não rodou `--past-cap`, ou já esgotou as rodadas extras. |
  | 5 | `STALE` | Há registro para outro conteúdo, ou para este conteúdo com `policy_sha` diferente, ou para este conteúdo com alguma linhagem bloqueante. |
  | 6 | `IN_PROGRESS` | Nenhuma das anteriores. |

  O relatório traz também `loop_state ∈ running|capped|converged`. Uma cobertura menor que a da primeira rodada do ciclo não é recusada: vira rebaixamento de ciclo (R24). `skipped` estrutural (`no_detailed_section`, `range_too_small`) não conta como redução.
- R26: no `loop next`, os árbitros das linhagens `contest_pending` (R29) rodam primeiro. Depois, se o `content` não mudou desde a rodada anterior **e** existe alguma linhagem bloqueante cujo status não muda sem edição ou `resolve` (`open`, `reopened`, `escalated`, `fixed` sem confirmação, ou `contest_pending` sem árbitro elegível), o tb não abre rodada nova e mostra `documento e fontes não mudaram desde a rodada N; há findings bloqueantes abertos`. Sai com exit 0 se algum árbitro rodou e com exit 2 se nenhum rodou. Nos demais casos, a rodada é aberta.
- R27 (orçamento): o tb conta as execuções de agente do ciclo (revisores, retries e árbitros) e para em `egm.max_agent_runs` (padrão 120). O que faltar vira `not_run` com `reason: budget`.
  - O pior caso de uma rodada é `revisores × 2 + contestações pendentes × 2`, e `--dry-run` imprime esse valor e o saldo.
  - Uma rodada cujo pior caso passe do saldo é recusada com exit 2.

**Resolve e árbitro**

- R28: `tb egm resolve <doc>`. Uma transição para o status atual é no-op com sucesso. Uma transição inválida dá exit 2 e não altera nada.

  | Flag | De | Para | Restrição |
  |------|----|------|-----------|
  | `--fix F-…` | open, reopened, escalated, contest_pending, settled, declined | fixed (sem confirmação) | Desde a `opened_round` (ou desde a rodada da transição, para `settled` e `declined`), houve um diff com alguma linha alterada que, ignorando espaços, tocou as linhas de `scope` mapeadas ou as linhas `evidence[].line ± 5` de uma fonte em `cited`. Os snapshots são os do R1, e a normalização de espaços é feita em Go sobre os hunks. A partir de `contest_pending`, cancela a contestação (`contests` continua 1). A partir de `settled`/`declined`, o rebaixamento de linhagem só sai com a confirmação do dono. Uma correção em outro trecho fecha pela reavaliação do dono, sem `--fix`. |
  | `--contest F-x --evidence path:line` | open, reopened | contest_pending | Motor efetivo com agente; `contests == 0`; o path está em `cited` e passa de novo pelo `resolveSource`; a persona não é comprehension. Se o path é o documento, a evidência vem do snapshot da rodada atual. Se é outra fonte, ela precisa estar versionada, com `hash-object --no-filters` igual ao blob de `HEAD`, e esse blob precisa ser igual ao da `opened_round`. |
  | `--decline F-…` | open, reopened | declined | `contests == 0`. Vale só para `nit` e `major`; um `major` recusado recebe rebaixamento de linhagem. Nunca vale para `blocker`. |
  | `--force-settle F-x --reason "…"` | open, reopened, contest_pending, escalated | settled | `forced: true`, rebaixamento de linhagem |

- R29 (árbitro): o árbitro recebe, nos blocos do R9:
  - o `message` e o `rationale` originais;
  - o `scope` inteiro, ou o `TRECHO-SINALIZADO`;
  - a evidência (±20 linhas), redigida e conferida pelo R17a.

  A saída segue `internal/egm/schema/arbiter.schema.json` (`{"nonce","checked":[{"heading","quote"}],"verdict":"withdraw|uphold","rationale"}`) e passa pela atestação do R17 (sobre o `scope` e a evidência), pelo R15a e pelo R16.

  Resultados:
  - `withdraw` de `nit`, de checker ou de `injection-scan`: `settled` (`by: arbiter`).
  - `withdraw` de `blocker` ou `major` com dono LLM: `fixed` sem confirmação. O dono reavalia na rodada seguinte: `still_holds: false` confirma; `true` reabre com `contests = 1`.
  - `uphold`: `open`, com `contests = 1`.
  - `not_run`: continua `contest_pending`. Depois de 2 `not_run` seguidos do árbitro, a linhagem vai para `escalated`.

  Depois de `uphold`, a linhagem só sai por `--fix`, por `still_holds: false` do dono ou por `--force-settle`.
- R30: `escalated` é um status definido só pelo tb (R17a, R29), sem flag de entrada. Uma linhagem blocker ou major em `escalated` só sai por `--fix` ou por `--force-settle`.

**Integração**

- R31 (CLI): todos os comandos aceitam `--json`.

  | Comando | Flags |
  |---------|-------|
  | `tb egm review <doc>` | `--engine`, `--reviewers`, `--dry-run`, `--restart` |
  | `tb egm loop next <doc>` | `--engine`, `--reviewers`, `--dry-run`, `--past-cap` |
  | `tb egm resolve <doc>` | as do R28 |
  | `tb egm status <doc>` | `--at <rev>`, `--strict` |

  - **`review`:** cria a rodada 1 do primeiro ciclo. Se já existe estado, sugere `loop next` ou `--restart`.
  - **`--restart`:** abre um ciclo novo, mantendo as linhagens, e rebaixa esse ciclo. O ciclo novo herda o orçamento já gasto. Se o ciclo atual está `capped`, `--restart` exige que o `content` tenha mudado.
  - **Config:** os comandos `tb egm` não passam pelo `config.Load()` do root (`cmd/tb/main.go`). A config de usuário é carregada com `config.LoadFile(config.GlobalConfigPath())`, ou com o arquivo de `-c`, e nunca vem da config mesclada nem de `Shared.FileConfig`. Por cima dela, só se aplica a config do repositório pelas regras do R34.
  - **MCP:** as tools `tb_egm_review`, `tb_egm_loop_next`, `tb_egm_resolve` e `tb_egm_status` ficam em `internal/registry/parity.go` e na lista de `parity_test.go`.
- R32 (códigos de saída):

  | Comando | 0 | 1 | 2 | 3 |
  |---------|---|---|---|---|
  | `review`, `loop next` | Rodada ou etapa de árbitros completa, e o veredito não é `FAIL` | `FAIL` | Erro de uso, de execução ou de env; orçamento; `loop next` sem mudança e sem árbitro (R26) | — |
  | `resolve` | Sucesso ou no-op | — | Transição inválida | — |
  | `status` | `PASS`, ou `PASS_PARTIAL` sem `--strict` | `PASS_PARTIAL` com `--strict` ou `egm.require_full_coverage: true`; `FAIL` | Erro de uso | `IN_PROGRESS`, `STALE`, `NO_STATE` |
- R33 (gate no `tb stack`): o `tb stack` ganha o op `egm`, as flags `--strict` e `--egm-doc <path>` (repetível), os campos `EgmDocs []string` e `Strict` em `stack.Options`, os argumentos `egm_docs` e `strict` no schema da `tb_stack` (com a descrição de `ops` incluindo `egm`) e o campo aditivo `warn` em `StepResult` e `Result`.
  - **Seleção:** o op escolhe os documentos sob gate (padrão `docs/specs/**` e `docs/adr/**`) adicionados ou modificados entre `merge-base(base, head)` e `head` (a mesma semântica de três pontos do stack), pelo `diff --name-status --no-renames` do R4c. Somam-se os de `--egm-doc`. Documentos apagados ficam de fora.
  - **Avaliação:** cada documento é avaliado com `status --at head`, com documento e fontes resolvidos pelo R7a.
  - **Seleção vazia:** `ok: true, warn: true`, com o aviso `no_gated_docs`, ou `ok: false` com `--strict`.

  | Veredito | Resultado no stack |
  |----------|--------------------|
  | `PASS` | `ok: true` |
  | `PASS_PARTIAL` | `ok: true, warn: true` (`ok: false` com `--strict` ou `require_full_coverage`) |
  | demais | `ok: false` |

  Com `warn`, o stack lista os rebaixamentos, e o exit é 0. Na Fase A, a escolha do range fica com quem chama o stack. A seleção por ref de confiança está em [egm trust](egm-trust.md).
- R33a (CI): o gate é local. Num checkout de CI não há estado, então o resultado é `NO_STATE` e `ok: false`. O CI pode rodar o loop com credenciais próprias ou não incluir o op `egm`.
- R33b (relatório): o `report.json` segue `internal/egm/schema/report.schema.json`, com os campos:
  - `slug`, `doc`, `doc_type`, `cycle`, `round`, `verdict`, `loop_state`, `content` e `target` (`worktree` ou `<rev>`);
  - `coverage`, `engine` e `engine_reason`;
  - `findings` por linhagem;
  - `skipped_sources`;
  - `warnings` (`config_untrusted`, `tools_used`, `prior_injection`, `unstaged_source`, `filtered_source`, `secret_like`…); o aviso `no_gated_docs` é do `tb stack`, não do relatório;
  - `downgrades` e `budget`.

  A ordenação é estável e o corpo não tem timestamps. O `report.md` é gerado a partir do JSON.
- R34 (config e compatibilidade): `review.Finding`, `tb review`, `tb drift`, `tb merge` e o router não mudam.

  **Chaves de execução e segredo:** `agent_cmd`, `path`, `env`, `home`, `vendor`, `output`, `tools_disabled`, `tools_check` e `forbidden_flags`. Vêm **só** da config de usuário.
  - Uma chave dessas presente no `.toolbelt/config.yaml` do repositório (no worktree ou, no modo gate, no blob de `head`) é ignorada, e o tb emite `config_untrusted`.
  - Com `-c/--config`, essas chaves são lidas do arquivo indicado, porque foi o próprio usuário quem escolheu o arquivo.

  **Chaves de limite:** `max_agent_runs`, `max_rounds`, `max_past_cap_rounds`, `max_parallel`, `timeout_per_reviewer`, `max_source_bytes`, `max_packet_bytes` e `max_stdout_bytes`. O repositório só pode **baixar** esses valores. Um valor do repositório acima do valor da config de usuário (ou do padrão) é limitado a ele, e o tb emite `config_untrusted`.

  **Chaves de política:** o repositório só pode **endurecer**.
  - `clean_rounds`, `attest.min_quotes`, `match.sim` e `match.overlap` só sobem.
  - `require_full_coverage` só vira `true`.
  - `doc_globs` e `detail_headings` só ganham itens.
  - `doc_types` e `refs_ignore` vêm só da config de usuário.
  - Valores que afrouxam são ignorados com `config_untrusted`.
  - No modo gate, a config do repositório vem do blob de `head`, com essa mesma regra.

  **Demais chaves:** vêm da config de usuário, sobreposta pela config do repositório.

  **Leitura da config do repositório:** o `tb egm` lê a seção `egm:` do `.toolbelt/config.yaml` com leitor próprio. A partir da raiz real, cada componente do caminho passa por `Lstat`, o arquivo precisa ser regular e o limite é 64 KiB. Fora disso, o arquivo é ignorado com `config_untrusted`. O `config.Load` do resto do tb não muda.

### Configuração

```yaml
egm:
  # Execução e segredo (R34): só na config de usuário.
  agent_cmd: []              # [0] absoluto; vazio → deterministic
  path: /usr/bin:/bin        # PATH do goldfish
  vendor: claude             # claude | custom
  output: claude-json        # claude-json | raw (padrão do custom: raw)
  tools_disabled: false      # obrigatório true com vendor: custom
  tools_check: auto          # auto (do perfil) | none (rebaixa)
  forbidden_flags: []
  env:
    pass: []                 # ex.: [ANTHROPIC_API_KEY]; prefixos proibidos no R4b
  home:
    copy: []                 # só se o R0 escolher cópia
    read_only: []            # diretórios de runtime do vendor
  # Limites (R34): o repositório só pode baixar max_* e timeout_per_reviewer.
  # Demais chaves: config de usuário sobreposta pela do repositório.
  doc_globs: ["docs/specs/**", "docs/adr/**"]
  doc_types: {"docs/adr/**": adr}   # resto → spec
  detail_headings: ["## Detailed Implementation Plan", "## Plano de Implementação Detalhado", "## Implementation Plan"]
  refs_ignore: ["refs/**"]
  match: {sim: 0.3, overlap: 0.5}   # 0 < x ≤ 1
  attest: {min_quotes: 3}    # ≥ 1
  require_full_coverage: false
  max_rounds: 5              # ≥ 1, por ciclo
  clean_rounds: 2            # 1 ≤ clean_rounds ≤ max_rounds
  max_past_cap_rounds: 3     # ≥ 0, por ciclo
  max_agent_runs: 120        # ≥ 1, por ciclo
  max_source_bytes: 65536
  max_packet_bytes: 409600
  max_parallel: 4            # ≥ 1
  timeout_per_reviewer: 5m   # time.ParseDuration, > 0
  max_stdout_bytes: 1048576
```

## Modelo de ameaças (núcleo)

| Ameaça | Mitigação |
|--------|-----------|
| Repositório de terceiros escolhe o binário ou encaminha segredos | R34 (execução e segredo só da config de usuário), R4b |
| Arquivo baixado com `.git/config`/`.gitattributes` hostis (filtros, fsmonitor, drivers, submódulos, `core.worktree`, `include`, `alternates`, `excludesFile`, `gitdir:` para outro repositório) | R4c (lista fechada, sem `git status`, sem varredura total, timeout), R4d (raiz real pelo filesystem, `GIT_WORK_TREE` explícito, inspeção prévia do `.git`) |
| Config do repositório que sobe limites (custo, timeout), ou que é um FIFO ou symlink | R34 (limites só baixam; leitura com `Lstat`, arquivo regular, 64 KiB) |
| Evidência de contestação apontando para segredo ou symlink | R28 (`resolveSource` de novo), glossário (`cited` só com caminhos verificados) |
| Loaders do `tb stack` rodando git sem endurecimento | **Fora do escopo** (Não-objetivos); use `tb egm status --at` |
| Symlink versionado, inclusive em diretório-pai, e índice forjado | R7 (`Lstat` por componente, modo no índice), R4d |
| Injeção de código no agente via env | R4b |
| Atualização do vendor religa as ferramentas | R15a |
| Documento, fonte ou `PRIOR` instrui os revisores | R17a, R17, R9 (`PRIOR` com campos fixos, sem texto do elephant). Risco residual: injeção sutil que o scan não pega, obedecida por todos, com atestação válida. |
| Usar ou convencer o árbitro | R17a, R29 |
| Goldfish lê `/proc/<pid>/environ` ou o repositório | R1 (restrição de ptrace da Landlock, confirmada no R0; workdir fora do repositório), R5 e R15a (sem ferramentas). |
| `CLAUDE.md` global | R4, R0 |
| Segredo no conteúdo | R7, R8 |
| Credencial vazada ou rotacionada | R0, R4 |
| Saída falsa no stderr, JSON truncado | R2 |
| Atalho desleixado (`--fix` sem corrigir, contestar até passar, rebaixar severidade, `--decline`/`--force-settle` lavado por edição, blocker absorvido por nit recusado, reverter para um conteúdo que já convergiu com blocker aberto) | R28, R29, R20 (severidade maior vira linhagem nova), R23, R24, R25 (linha 2 exige zero bloqueantes abertos), R26 |
| Convergência sobre textos diferentes, trocar o conteúdo depois da revisão | R23, R25, R33 (`status --at head`) |
| Crash no meio de uma escrita, ou dois processos ao mesmo tempo | R21 (escrita atômica, `flock` não bloqueante) |
| Config do repositório afrouxando a política | R34 (o repositório só endurece) |
| `refs-exist` consultando paths fora do repositório | R13 (`resolveSource` antes da existência) |
| Custo sem teto (`loop next` repetido, `--restart`) | R26, R27, R31 (`--restart` herda o orçamento) |
| Fraude ativa do mesmo usuário | **Fora do escopo**: ver [egm trust](egm-trust.md) |

## Requisitos não funcionais (NFRs)

- **Determinismo:** no motor `deterministic`, as mesmas entradas geram o mesmo `report.json` byte a byte.
- **Custo:** cerca de 9 execuções por rodada com DetailedRange e 6 sem, limitadas pelo R27.

## Alternativas

Ver o [ADR-014](../adr/014-egm-nativo.md#alternativas-consideradas).

## Questões em aberto

- O R0 define: autenticação, flags sem ferramentas, detecção de uso de ferramenta e diretórios de runtime do `claude`, limiares de matching e taxa de convergência.

## Plano de Implementação Detalhado

| Arquivo | Ação | Motivo |
|---------|------|--------|
| `internal/sandbox/sandbox.go`, `internal/sandbox/landlock_linux.go`, `internal/sandbox/landlock_other.go` | Modify | Campos novos e máscara de arquivo (R2) |
| `bench/egm-spike/` | Create | Spike (R0) |
| `docs/campo/dogfood.md` (+ docs/en/) | Modify | Resultado do R0 |
| `internal/egm/vendor.go` | Create | Perfis, autenticação, flags, envelope, detecção de ferramenta, recusa de bypass (R4, R5, R15, R15a) |
| `internal/egm/env.go` | Create | Env mínimo (R4b) |
| `internal/egm/run.go` | Create | Registry único, subreaper, atribuição de órfãos, timeout, retry (R3, R16) |
| `internal/egm/git.go` | Create | Git do egm: opções, env e subcomandos permitidos (R4c) |
| `internal/egm/repo.go` | Create | Raiz real do repositório, inspeção do `.git` e `repo-id` (R4d, R21) |
| `internal/egm/glob.go` | Create | Globs com `**` para `doc_globs`, `doc_types` e `refs_ignore` |
| `cmd/tb/main.go` | Modify | Pular o `config.Load()` do root para os comandos `tb egm` (R31) |
| `internal/egm/packet.go`, `internal/egm/sources.go` | Create | Snapshot, conteúdo do worktree ou do commit, `resolveSource`, exclusões, prompt (R1, R7, R9) |
| `internal/graph/index_docs.go` | Modify | Exportar `CollectMDLinks`; criar `CollectBacktickSpans`; extensões novas (R7) |
| `internal/egm/redact.go` | Create | Redação do egm com preservação de linhas (R8) |
| `internal/egm/personas/*.md`, `internal/egm/standards/*.md` | Create | 8 personas e `arbiter`; standards `feature-spec`, `adr`, `implementation-readiness`, `zoom-out` (R10, R11, R13, R29) |
| `internal/egm/checkers/refs.go`, `internal/egm/checkers/sections.go`, `internal/egm/checkers/planfiles.go` | Create | Checkers (R13) |
| `internal/egm/injection.go`, `internal/egm/injection_patterns.txt` | Create | Scan (R17a) |
| `internal/egm/attest.go` | Create | Atestação (R17) |
| `internal/egm/stopwords.txt` | Create | Stopwords (R18) |
| `internal/egm/finding.go`, `internal/egm/validate.go`, `internal/egm/schema/finding.schema.json`, `internal/egm/schema/arbiter.schema.json` | Create | Tipos, validador, schemas, `unverified` (R11, R15–R16a) |
| `internal/egm/match.go` | Create | Matching, dono, transferência de dono, linhagem (R18–R20) |
| `internal/egm/state.go` | Create | Estado atômico, `flock`, ciclos, convergência por conteúdo, rebaixamentos, vereditos, orçamento (R21–R27) |
| `internal/egm/slug.go` | Create | Slug (R14) |
| `internal/egm/report.go`, `internal/egm/schema/report.schema.json` | Create | Relatório (R33b) |
| `internal/egm/resolve.go` | Create | Transições e árbitro (R28–R30) |
| `internal/egm/review.go` | Create | Orquestração por fases, tipos, ranges, etapa de árbitros, `--restart` (R6, R10, R12, R26, R31) |
| `internal/egm/config.go` | Create | Leitor da config do repositório, classes de chave, limites e `config_untrusted` (R34) |
| `internal/egm/testdata/paraphrases.json` | Create | Corpus do R0 |
| `internal/config/config.go` | Modify | `EgmConfig` com Validate (R34) |
| `cmd/tb/egm.go` | Create | Grupo cobra (R31, R32) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | Op `egm`, `EgmDocs`, `Strict`, `--strict`, `--egm-doc`, `warn` (R33) |
| `internal/registry/parity.go`, `internal/registry/parity_test.go` | Modify | Tools `tb_egm_*`, schema da `tb_stack` (R31, R33) |
| `internal/bench/fake_agent_egm.go`, `cmd/tb/fake_agent_egm.go` | Create | Modos egm e o subcomando `__fake-agent-egm`, só com build tag `egmfake` (ver a nota abaixo) |
| `docs/cli.md`, `docs/arquitetura.md` (+ docs/en/) | Modify | Documentar `tb egm` (na seção 8.5 de `docs/arquitetura.md`); trocar o exemplo fictício `tb review pr:42` |
| `CHANGELOG.md` | Modify | Entrada não lançada |

Nota sobre o fake-agent:
- **Build:** os modos egm ficam em `internal/bench/fake_agent_egm.go`, com a build tag `egmfake`. Os testes e2e compilam um binário separado (`go build -tags egmfake -o <tmp>/fake-agent ./cmd/tb`) e o configuram como `egm.agent_cmd` (`[<tmp>/fake-agent, __fake-agent-egm, <dir-ro>/fake.json]`) com o perfil `claude`. `<dir-ro>` é um diretório listado em `egm.home.read_only`.
- **Entrada:** `__fake-agent-egm` é um subcomando oculto novo, registrado só pelo arquivo com a tag, com `DisableFlagParsing: true`. O primeiro argumento é o arquivo de config; o resto (os flags do perfil `claude`) é ignorado. O workdir é o cwd, que é o pacote. Ele lê o prompt do stdin e extrai `REVIEWER:`, `NONCE:`, `PRIOR:` e os blocos.
- **Saída:** devolve o envelope `claude-json`. Nele, `num_turns` e `tool_use` são controlados pelo modo.
- **Configuração:** um JSON `{"<revisor ou arbiter>": ["<modo tentativa 1>", "<modo tentativa 2>"]}`, com a tentativa vinda de `TB_EGM_ATTEMPT`. Qualquer erro de leitura ou de formato é falha dura. O `__fake-agent` atual não muda.
- **Modos** (cada um emite o envelope `claude-json`):
  - `findings`: um finding `major` com evidência no documento e atestação válida;
  - `clean`: zero findings, atestação válida e `prior` com `still_holds: false` para tudo;
  - `invalid`, `no-nonce`, `over-severity`, `omit-prior`, `bad-attest`: a falha do nome;
  - `stderr-json`: JSON falso no stderr e saída válida no stdout;
  - `overflow`: stdout acima do limite;
  - `sleep`: dorme além do timeout;
  - `probe-fs`: tenta ler `$HOME` real, o repositório, `/proc/<ppid>/environ` e outro pacote, e relata cada resultado num finding `nit`;
  - `escape`: deixa um processo `setsid` sem `TB_EGM_OWNER`;
  - `tool-use`: `num_turns: 2`;
  - `withdraw` e `uphold`: saída de árbitro.

### Plano de verificação

- **R0:** script em `bench/egm-spike/`. O resultado vai em `docs/campo/dogfood.md` (PT e EN), e o corpus e a taxa de convergência em `internal/egm/testdata/paraphrases.json`.
- **Sandbox:**
  - pacote como workdir RO e repositório inacessível;
  - `PrivateTmp`;
  - precedência de `Home` e `Path`;
  - stdout separado e overflow;
  - regra de arquivo sem EINVAL;
  - `probe-fs` e `escape`, inclusive em paralelo;
  - órfão sem owner;
  - subreaper indisponível.

  Um skip só é legítimo sem Landlock.
- **Conteúdo hostil (CUJ-5, CUJ-6):**
  - `.git/config` com `core.fsmonitor`, `diff.external`, `filter.x.clean`, `include.path`, `core.sshCommand` e `core.worktree`;
  - `.gitattributes` com `* filter=x`;
  - submódulo com config própria;
  - `.git` como arquivo `gitdir:` apontando para fora;
  - índice forjado com um diretório-pai symlink para fora;
  - symlink para `/dev/zero` e um FIFO versionados;
  - repositório com `egm.agent_cmd` e `egm.env.pass`;
  - `.toolbelt/config.yaml` como FIFO, como symlink para `/dev/zero` e com limites acima dos do usuário;
  - `[include]`, `alternates`, `core.excludesFile` apontando para um FIFO, `.git` como symlink e `gitdir:` sem back-pointer;
  - git travado (timeout).

  Em todos os casos, `review`, `status --at` e `resolve` não executam nada, não leem nada fora da raiz e não travam. Um teste estático confere que o pacote `egm` só chama os subcomandos do R4c. Os testes de `tb merge` e do router continuam passando sem alteração.
- **Env e ferramentas:**
  - `NODE_OPTIONS` e afins não chegam ao goldfish;
  - prefixo proibido em `egm.env.pass` é recusado;
  - modo `tool-use` dá `not_run` com `tools_used`.
- **Atalhos (tabelas):**
  - `--fix` sem mudança, ou só com espaços;
  - segunda `--contest`;
  - evidência fora de `cited`, alterada ou com injeção;
  - termo de árbitro perto do match;
  - `match_sha` mudado reabre a linhagem;
  - `--decline` de blocker recusado;
  - nit recusado seguido de um blocker que casa com ele: nova linhagem, sem convergência;
  - reverter para um conteúdo convergido com blocker aberto dá `STALE`;
  - `--contest` offline recusado; `--fix` a partir de `contest_pending`; dois `not_run` do árbitro levam a `escalated`;
  - `--force-settle`, depois `--fix` com correção real e confirmação do dono: volta a `PASS`;
  - `loop next` repetido com linhagem `escalated` e sem edição é recusado;
  - `--restart` herda o orçamento e, com o ciclo `capped`, exige `content` novo;
  - repositório que afrouxa `clean_rounds`, `match`, `doc_types` ou `refs_ignore` é ignorado;
  - `refs-exist` com `..` e com symlink: ignorados, sem relatar existência;
  - deny-list estendida e `secret_like`;
  - lock ocupado: exit 2; `status` lê durante uma rodada;
  - `.git` de repositório recém-criado, sem `index`, e `index` grande são aceitos;
  - evidência de contestação com symlink ou `.env` é recusada;
  - `policy_sha` alterado dá `STALE`;
  - spec sem plano dá `PASS_PARTIAL`;
  - `capped` sem `--past-cap` dá `FAIL`;
  - `--force-settle`, edição e nova convergência: continua `PASS_PARTIAL`;
  - injeção em prosa, código, citação, fonte e `PRIOR`;
  - goldens de injeção desta spec e do ADR-014;
  - atestação inválida, só com headings, válida no DetailedRange desta spec, e `range_too_small`;
  - CUJ-7 e CUJ-8;
  - duas rodadas limpas sobre conteúdos diferentes não convergem;
  - `withdraw` de blocker reaberto pelo dono;
  - estreitamento de cobertura;
  - exclusão `max_*`;
  - dono `skipped` transfere a linhagem;
  - gate com documento alterado sem `PASS`, documento apagado, seleção vazia e renomeação.
- **Match:**
  - corpus do R0;
  - linhas deslocadas;
  - heading renomeado;
  - dono por severidade;
  - `style-feature` × `style-detailed`;
  - fechamento por rerun.
- **Estado:**
  - transições do R28 e idempotência;
  - precedência dos vereditos (R25);
  - ciclos;
  - `capped`, `--past-cap` e `max_past_cap_rounds`;
  - R26 com e sem árbitros;
  - árbitro `withdraw`, `uphold` e `not_run`;
  - orçamento;
  - processo morto com o lock (o lock é liberado);
  - kill entre o temporário e o `rename` (o estado anterior continua válido);
  - `NO_STATE` num checkout limpo.
- **Tipos de documento:**
  - um ADR sem plano chega a `PASS`;
  - uma spec sem plano chega a `PASS_PARTIAL`;
  - um ADR com plano exige os 9 revisores e mantém o standard `adr`.
- **Golden:** o `report.json` desta spec no motor `deterministic`, incluindo o parsing do próprio plano (R13, com a diretiva de espelho) e o `refs-exist` sem falsos positivos.
- **Regressão:**
  - JSON de `tb review` e `tb drift`;
  - `tb mem` e `tb_mem_*` inalterados (R8);
  - arestas de menção e de link do `tb graph` para MD, TXT e HTML (R7);
  - `go test ./...` e `TestDocsEnglishMirrorComplete`.

*Elaborado com base no padrão EGM*
