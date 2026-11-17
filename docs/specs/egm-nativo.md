🇧🇷 Português | [🇺🇸 English](../en/specs/egm-nativo.md)

# Spec: EGM nativo, Fase A (`tb egm review|loop|resolve|status`)

Autor: toolbelt · Status: Rascunho (rodada de revisão 4) · Data: 2026-11-17 · Decisão: [ADR-014](../adr/014-egm-nativo.md)

## Visão geral

O `tb egm` revisa **documentos de design** com goldfish (revisores em contexto fresco) e roda o loop de revisão até convergir. A Fase A cobre `review`, `loop next`, `resolve` e `status`, com o preset `consolidated`, o perfil de vendor `claude` e um perfil `custom`. Drift-report, bootstrap, draft, brainstorm, outros presets e o perfil `codex` ficam para specs próprias.

## Glossário

| Termo | Definição |
|-------|-----------|
| elephant | Quem escreveu o documento e chama o `tb egm`: o agente principal ou um humano. Tem shell como o mesmo usuário (R37). |
| goldfish | Revisor one-shot em processo isolado. Vê só o próprio prompt. |
| revisor | Vaga do preset (R10), como `style-feature`. Cada revisor usa uma persona; uma persona pode alimentar mais de um revisor. |
| revisor dono | O revisor que criou uma linhagem. É o único que a reavalia (R20). |
| árbitro | Goldfish que julga uma contestação (R31). |
| persona | Lente do revisor: o que procurar e como julgar (`personas/<nome>.md`). |
| standard | Critério de qualidade do documento usado pela persona (`standards/<nome>.md`), como `feature-spec`. |
| preset `consolidated` | Rodada com os 9 revisores do R10 (8 personas) em paralelo, sem checkpoints. |
| pacote | Registro em disco do que foi entregue a um goldfish (R1). |
| nonce | Valor aleatório de 128 bits por execução. Delimita os blocos de dado e precisa ser ecoado. Prova que a saída veio desta execução, **não** que o modelo resistiu a injeção. |
| atestação | Lista `checked` de trechos literais do documento que o revisor declara ter lido. O tb confere cada trecho (R17b). |
| slug | Identificador do documento revisado (R14). |
| FeatureRange / DetailedRange | Linhas antes e a partir do heading de plano detalhado (R6). |
| linhagem `F-<k>` | ID estável de um finding entre rodadas (`k` sequencial por slug). |
| ocorrência `<reviewer>-r<round>-<i>` | ID de um finding numa rodada. |
| status de revisor | `completed` (rodou com saída válida), `not_run` (falhou após retry, R16), `unavailable` (persona LLM sem agente) ou `skipped` (sem range aplicável, R6). |
| perfil de vendor | Receita por CLI: argv base, flags sem ferramentas, autenticação, env e envelope de saída (R4, R5, R15). |
| fonte de confiança | Origem das chaves `egm.*` sensíveis: config de usuário fora do repo ou a config no ref de confiança (R37). |
| confirmação humana | Confirmação digitada num TTY interativo, registrada no log de auditoria (R38). |

## Problema

O `tb review` revisa diffs com heurísticas. Não revisa documentos, não tem revisor LLM e não converge. O EGM upstream faz isso em prosa, com isolamento, estado e aprovação a cargo do harness e do modelo. Ver o [ADR-014](../adr/014-egm-nativo.md#contexto).

## Objetivos

- G1: isolamento por processo, sandbox, HOME descartável e ausência de ferramentas.
- G2: finding estruturado (`egm.Finding`) consumível por `tb stack` e MCP.
- G3: convergência decidida pelo tb e persistida. O elephant não chega a `PASS` sem revisão real: nem por loteria, injeção, config adulterada ou estado editado à mão. Isso vale dentro dos limites do R37.
- G4: modo offline útil, com cobertura declarada, que não passa num gate estrito.
- G5: CLI e JSON de `tb review` e `tb drift` **inalterados**.

## Não-objetivos

- O tb não pergunta nada ao usuário (exceto a confirmação humana do R38) e não edita o documento revisado.
- O tb não altera a severidade dada por um revisor. Saída fora do teto da persona é inválida (R11).
- O tb não fala com rede nem chama API de LLM.
- Resistir a um processo do mesmo usuário com intenção ativa de fraudar (ler a chave de auditoria, forjar TTY). O R37 declara o limite.
- Fora desta fase: presets `staged`/`feature`/`implementation`, personas por path, `intermediate-zoom-out`, perfil `codex`, churn, adaptador `review.Finding` → `egm.Finding`, tokens de capacidade do ADR-005 §4, drift-report, bootstrap, draft e brainstorm.
- O texto das personas do upstream não é copiado.

## Jornadas de usuário (CUJs)

- **CUJ-1.** *Dado* `docs/specs/x.md` e o perfil `claude` na fonte de confiança, *quando* rodo `tb egm review docs/specs/x.md`, *então* 9 goldfish rodam isolados e recebo `round-1/report.{md,json}` com o veredito `IN_PROGRESS`.
- **CUJ-2.** *Dado* que não há agente, *quando* rodo o mesmo comando, *então* rodam `refs-exist`, `required-sections` e `plan-files-listed`, as personas LLM ficam `unavailable` e o loop só pode terminar em `PASS_PARTIAL`.
- **CUJ-3.** *Dado* que a rodada 1 tem findings, *quando* edito o documento, rodo `tb egm resolve docs/specs/x.md --fix F-1,F-2` e depois `tb egm loop next docs/specs/x.md`, *então* o revisor dono de cada linhagem confirma se ela ainda vale, e o tb reporta `converged: true` após 2 rodadas limpas seguidas.
- **CUJ-4.** *Dado* o blocker `F-3`, *quando* rodo `tb egm resolve docs/specs/x.md --contest F-3 --evidence internal/x.go:40` (arquivo versionado e citado pelo documento), *então* F-3 fica `contest_pending`. No próximo `loop next`, um árbitro fresco decide entre `settled` e `open`. Uma segunda contestação de F-3 é recusada.
- **CUJ-5.** *Dado* que eu (elephant) edito `.toolbelt/config.yaml` no worktree para `clean_rounds: 1`, *quando* rodo `loop next`, *então* o tb ignora o valor, usa o da fonte de confiança e emite o aviso `config_untrusted`.

## Plano técnico

```
elephant ──tb egm review|loop next|resolve|status──┐
                                                    ▼
┌───────────────────────────── tb ─────────────────────────────┐
│ config de confiança (user / ref) → snapshot round-<n>        │
│ fontes (links+paths) → deny-list → redact → prompt (nonce)   │
│        │                                                     │
│        ├─ llm: sandbox.New(RO workdir, PrivateTmp, Home,     │
│        │        Stdin, Stdout separado) × 9, sem ferramentas │
│        └─ deterministic: refs-exist, required-sections,      │
│                          plan-files-listed                   │
│        ▼                                                     │
│ validate (schema, teto, nonce, atestação, evidência)         │
│        ▼                                                     │
│ match → linhagem → registro da rodada (HMAC encadeado)       │
│        ▼                                                     │
│ state.json + report.{md,json} → veredito recalculado         │
└──────────────────────────────────────────────────────────────┘
```

### Requisitos (EARS)

**R0: Spike.** Bloqueia R1–R5 e a passagem do ADR-014 para `accepted`. Um protótipo em `bench/egm-spike/` deve provar que `claude -p`:

1. roda em `sandbox.New` com workdir só-leitura, `TMPDIR` privado e HOME descartável;
2. recebe o prompt por stdin e devolve JSON em stdout separado do stderr;
3. roda **sem nenhuma ferramenta**;
4. não carrega `~/.claude/CLAUDE.md`, as memórias nem os settings do usuário;
5. produz uma atestação (R17b) válida.

O spike também deve:

- determinar a forma de autenticação do perfil `claude`, nesta ordem de preferência:
  1. API key via `egm.env.pass`;
  2. diretório de config do vendor apontado para o HOME descartável (por exemplo, `CLAUDE_CONFIG_DIR`);
  3. cópia de arquivo de credencial, só em último caso;
- testar se um refresh de token dentro do HOME descartável invalida a credencial real;
- listar os diretórios que o binário precisa ler (o interpretador do shebang e `node_modules`);
- registrar o macOS (Keychain) como limitação;
- coletar ao menos 30 pares de findings parafraseados para calibrar R18–R20.

O resultado vai em `docs/campo/dogfood.md` (PT e EN). Se o spike falhar, o ADR não passa para `accepted` e o achado fica registrado no ADR.

**Sandbox e execução**

- R1: Em cada rodada, o tb deve gravar `.toolbelt/egm/<slug>/round-<n>/target.md` (snapshot redigido) em qualquer motor. Para cada goldfish LLM, deve gravar também `round-<n>/packets/<reviewer>/` com:
  - `prompt.txt`;
  - `sources.json` (path, sha256 do blob, linhas incluídas);
  - `meta.json` (persona, standard, range, nonce, `argv_sha256`, `binary_sha256`).
- R2: `sandbox.Config` deve ganhar os campos abaixo. Com eles zerados, nada muda.
  - `ReadOnlyWorkDir`: a regra do workdir fica só-leitura.
  - `PrivateTmp string`: diretório gravável. Substitui a regra de `os.TempDir()` e é exportado em `TMPDIR`.
  - `Home string`: diretório gravável. Exportado em `HOME`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME` e `XDG_DATA_HOME`, sobrescrevendo valores vindos de `Config.Env`.
  - `Stdin io.Reader`.
  - `Stdout io.Writer` com `MaxStdoutBytes int`: o stdout vai só para esse writer. Se passar do limite, a captura para e o resultado leva `StdoutOverflow: true`, sem truncamento silencioso. O stderr continua no `tailBuffer`.
  - Correção em `addRule`: quando o path é um arquivo regular, a máscara passa a ser `ReadFile|Execute`, sem `ReadDir`. Hoje uma regra de arquivo com `ReadDir` falha com EINVAL.
- R3: O goldfish deve chamar `sandbox.New` diretamente:
  - com `ProcessGroup: true` e um `sandbox.NewRegistry("TB_EGM_OWNER")` próprio;
  - com `GroupOwner = egm:<slug>:r<n>:<reviewer>` e essa mesma string em `TB_EGM_OWNER` no env do filho;
  - com `sandbox.AcquireSubreaper()` uma vez por processo `tb egm`.

  Depois de cada revisor, o tb chama `Registry.Sweep()`:
  - órfão com o owner daquele revisor: o revisor vira `not_run` com `reason: escaped_process`;
  - órfão sem owner atribuível: a rodada é abortada (exit 2), e nada é gravado no estado.

  O timeout vem de `context.WithTimeout(egm.timeout_per_reviewer)`. O motor `llm` exige Linux com Landlock (fail-closed).
- R4: Para o perfil `claude`, o tb deve:
  1. criar o HOME e o `PrivateTmp` em `os.TempDir()/tb-egm-<rand>/`, com modo `0700`;
  2. autenticar pela forma escolhida no R0: as variáveis de `egm.env.pass` (somadas a `env.pass` global, depois de `execenv.Filter`), ou o config dir no HOME descartável, ou uma cópia `0600` de `egm.home.copy`;
  3. liberar só-leitura o diretório do binário resolvido (`EvalSymlinks`), o diretório do interpretador do shebang e os diretórios de `egm.home.read_only`;
  4. apagar tudo no fim, com defer, inclusive em timeout. Na inicialização e a cada rodada, varrer os `tb-egm-*` órfãos com mais de 1h.
- R5: O argv do goldfish é `egm.agent_cmd` mais os flags do perfil, que desligam **todas** as ferramentas. Todo o conteúdo vai no prompt. O tb recusa (exit 2):
  - argv com `bypassPermissions`, `--dangerously-skip-permissions` ou flags de `egm.forbidden_flags`;
  - perfil `custom` sem `egm.tools_disabled: true`.

  Essas chaves só são lidas da fonte de confiança (R37).
- R6: O tb deve dividir o documento na **primeira ocorrência, em ordem de linha**, de qualquer heading de `egm.detail_headings`. Sem ocorrência, os revisores do DetailedRange ficam `skipped` com `reason: no_detailed_section`. `skipped` conta como cobertura parcial (R25).

**Fontes, redação e prompt**

- R7: As fontes do pacote vêm de dois lugares:
  - (a) links markdown relativos (`CollectMDLinks` + `ResolveDocHref`);
  - (b) spans entre crases (`CollectBacktickSpans`, extraído de `reBacktick`) e paths em texto (`CollectPathLikes`), filtrados por `LooksLikePath`.

  A lista de extensões de `rePathLike` deve ganhar `json`, `yaml`, `yml` e `toml`, e paths terminados em `/` passam a valer como diretório (diretório não entra como fonte, só é verificado no R13). Tudo isso é exportado de `internal/graph/index_docs.go`, junto com `NormalizeRelPath`. O `egm.resolveSource` só acrescenta a recusa de escape da raiz e de path absoluto.

  O próprio documento é sempre fonte. O tb deve excluir, com motivo em `skipped_sources`:
  - paths fora do repositório e URLs;
  - arquivos não versionados ou ignorados;
  - `.env*`, `*.pem`, `*.key`, `*.p12` e `id_*`;
  - `.toolbelt/`;
  - `**/testdata/**`, `internal/egm/personas/` e `internal/egm/standards/`;
  - arquivos acima de `egm.max_source_bytes`;
  - o que exceder `egm.max_packet_bytes`.

  Só os revisores com "fontes de código" (R10) e o árbitro recebem fontes além do documento.
- R8: `mem.RedactText` deve ganhar os padrões abaixo e preservar a contagem de linhas (cada match vira `[REDACTED]` mais as quebras de linha originais). A mudança afeta a saída de `tb mem` e é coberta por teste.
  - `\bsk-ant-[A-Za-z0-9_-]{20,}`
  - `\bsk-[A-Za-z0-9]{32,}`
  - `(?i)\b[A-Za-z0-9_]*_(KEY|SECRET|PASSWORD)=[^\s"'&]+`
- R9: `prompt.txt` deve ter, nesta ordem:
  - persona e standard;
  - `RANGE: <a>-<b>`;
  - `NONCE: <nonce>`;
  - o contrato de saída (schema);
  - `PRIOR:` seguido de JSON com as linhagens das quais o revisor é dono (R20);
  - os blocos `<<<DATA-<nonce> path=…>>> … <<<END-<nonce>>>>`, com a numeração de linha original.

  O prompt declara que o conteúdo dos blocos é dado e nunca instrução.

**Personas, standards e motores**

- R10: O preset `consolidated` roda em paralelo, até `egm.max_parallel`:

  | Revisor | Persona | Standard | Range | Fontes de código |
  |---------|---------|----------|-------|------------------|
  | zoom-out | zoom-out | zoom-out | doc inteiro | não |
  | inventive-catalyst | inventive-catalyst | (nenhum) | doc inteiro | não |
  | comprehension | comprehension | (nenhum) | doc inteiro | não |
  | design | design | feature-spec | FeatureRange | não |
  | structural | structural | feature-spec | doc inteiro | não |
  | style-feature | style | (nenhum) | FeatureRange | não |
  | implementation-readiness | implementation-readiness | implementation-readiness | DetailedRange | sim |
  | codebase-details | codebase-details | implementation-readiness | DetailedRange | sim |
  | style-detailed | style | (nenhum) | DetailedRange | não |

  `--reviewers a,b` restringe a lista e torna a cobertura parcial (R25).
- R11: Personas, `arbiter` e standards são embutidos (`go:embed`) em `internal/egm/personas/` e `internal/egm/standards/`, com texto próprio. Cada persona declara `max_severity`. Para `inventive-catalyst`, é `major`. Um finding acima do teto torna a saída inválida (R16).
- R12: `--engine auto|llm|deterministic`, padrão `auto`.
  - `auto` usa `llm` se `egm.agent_cmd` (da fonte de confiança) estiver configurado e for executável. Senão, usa `deterministic`.
  - Não há herança de `router.agent_cmd`.
  - `--engine llm` sem agente é erro (exit 2).
  - Override por revisor: `egm.reviewers.<nome>.agent_cmd`, também lido da fonte de confiança.
- R13: No motor `deterministic`:

  | Checker | Cobre parcialmente | Verifica |
  |---------|--------------------|----------|
  | `refs-exist` | codebase-details | paths e links do R7 existem (diretórios inclusive). Com `TB_GRAPH=on`, também confere no `tb graph` identificadores com forma de símbolo: `pkg.Ident`, `Type.Method` ou CamelCase com 2+ maiúsculas. Sem o grafo, não confere símbolos. |
  | `required-sections` | structural | headings exigidos pelo standard `feature-spec` |
  | `plan-files-listed` | implementation-readiness | parsing a seguir |

  O parsing do `plan-files-listed` é este:
  - **Tabela:** a primeira tabela sob o DetailedRange que tenha uma coluna `Arquivo`/`File` e uma `Ação`/`Action`.
  - **Arquivos da célula:** só os spans entre crases, separados por vírgula.
  - **Expansões:** `{a,b}` vira os dois paths; `(+ docs/en/)` gera o espelho de cada path `docs/…`.
  - **Glob `*`:** precisa casar com ao menos um arquivo existente se a ação for `Modify`.
  - **Reticências `…`:** casam por prefixo e precisam resolver para um path único.
  - **Regras de ação:** o valor tem de estar em `Create|Modify|Delete`, e todo `Modify` ou `Delete` precisa existir.

  As outras personas ficam `unavailable`.

**Contrato de saída e validação**

- R14: O slug é o path relativo em minúsculas, com `/` virando `--` e `[^a-z0-9-]` virando `-`, mais `-` e os 8 primeiros hex do sha256 do path. Para renomeações, ver R33.
- R15: O stdout, depois de remover o envelope do perfil, deve ser um objeto válido contra `internal/egm/schema/finding.schema.json`. O envelope depende de `egm.output`: `claude-json` usa o campo `result`, que é o padrão do perfil `claude`; `raw` é o padrão do `custom`. O objeto:
  `{"reviewer","nonce","checked":[{"heading","quote"}],"findings":[{"severity","scope":{"heading","lines":[a,b]},"message","rationale","evidence":[{"path","line"}]}],"prior":[{"lineage","still_holds":bool,"note"}]}`.
- R16: O tb tenta de novo uma vez e depois marca `not_run` com `reason` quando acontece qualquer um destes:
  - saída ausente;
  - schema inválido ou severidade acima do teto;
  - nonce errado;
  - atestação inválida (R17b);
  - `prior` incompleto (R20);
  - `StdoutOverflow`;
  - timeout;
  - processo escapado.

  `not_run` nunca equivale a zero findings.
- R17: Um finding é marcado `unverified: true` quando `evidence.path:line` não está nas fontes do pacote (o documento inclusive) ou quando `scope.heading` não existe no snapshot.
- R17a (scan anti-injeção): antes de cada rodada, o tb varre o snapshot. O scan:
  - **ignora** blocos de código, código inline e citações (`>`);
  - procura só padrões dirigidos ao revisor ou modelo (lista PT+EN embutida em `internal/egm/injection_patterns.txt`). Exemplos (aqui em código inline justamente para não disparar o scan): `ignore (as|all) (previous|prior|above) instructions`, `do not report`, `return (no|empty|zero) findings`, `you are (approved|pre-approved)`, `como revisor, (não|aprove)`, `output {`, `responda apenas`;
  - ao achar um padrão, emite um finding `major` do revisor `injection-scan`, com as linhas como `scope`. Esse finding é contestável no árbitro (R31).

  O scan também roda sobre o excerto de evidência antes de enviá-lo ao árbitro: se houver match, a contestação é recusada. Esta própria spec é um caso golden **negativo** e não pode disparar o scan.
- R17b (atestação): cada revisor LLM deve devolver em `checked` ao menos `max(3, ceil(H/3))` entradas, em que `H` é o número de headings no seu range.
  - Cada `quote` tem de 20 a 200 caracteres e aparece **literalmente** (após normalizar espaços) no snapshot, sob o heading declarado e dentro do range do revisor.
  - As entradas precisam cobrir headings distintos.
  - Uma atestação abaixo do mínimo, ou com algum trecho que não confere, é inválida (R16).

**Matching, linhagem e estado**

- R18: Definições:
  - `normalize(msg)`: minúsculas, sem acentos, sem pontuação, espaços colapsados e sem stopwords (lista PT+EN embutida).
  - `sim(a,b)`: Jaccard dos tokens de `normalize`.
  - `overlap(r1,r2)`: `|r1∩r2| / min(|r1|,|r2|)`.

  Os limiares ficam em `egm.match.sim` (padrão 0.3) e `egm.match.overlap` (padrão 0.5), calibrados com o corpus do R0.
- R19: Na mesma rodada, dois findings se fundem quando `sim ≥ egm.match.sim` **e** (`overlap ≥ egm.match.overlap`, ou, sem `lines`, o mesmo `heading`). A fusão mantém a maior severidade e a lista `reviewers[]`. O revisor dono é o da primeira ocorrência, na ordem do R10.
- R20: Entre rodadas, a fonte primária é a **reavaliação pelo revisor dono**. As linhagens `open`, `reopened` e `fixed` de um revisor entram em `PRIOR` (R9) só para ele, que responde `prior[].still_holds`:

  | Status atual | `still_holds: true` | `still_holds: false` |
  |--------------|---------------------|----------------------|
  | open / reopened | continua | → fixed (`by: reviewer`) |
  | fixed | → reopened | continua fixed |

  Omitir uma linhagem de `PRIOR` invalida a saída. Linhagens de checkers determinísticos e do `injection-scan` são reavaliadas pela re-execução do checker:
  - não reproduzida: `open` ou `reopened` → `fixed` (`by: rerun`);
  - reproduzida: `fixed` → `reopened`.

  Findings novos são casados com as linhagens pelo mapeamento de linhas (`git diff --no-index -U0` entre `round-<n-1>/target.md` e `round-<n>/target.md`) e pela regra do R19. Um finding novo que casa com uma linhagem `fixed` a reabre. Um que não casa vira uma nova `F-<k>`.
- R21: `state.json` (`0600`) contém:
  - **registros de rodada:** `n`, `doc_sha256` (original), `head_commit`, `dirty`, `sources` (path → sha256 do blob), motor, revisores e status, `argv_sha256`, `binary_sha256`, `config_source`, `prev_hmac` e `hmac` (R38);
  - **mapa:** ocorrência → linhagem;
  - **linhagens:** `severity`, `owner`, `persona`, `status ∈ open|fixed|reopened|contest_pending|settled|escalated|declined`, `contests` e `origin_round`;
  - **histórico de transições:** comando, rodada, motivo e `by ∈ elephant|reviewer|rerun|arbiter|human`.

  `.toolbelt/egm/` entra em `config.LocalCacheExcludePatterns`.
- R22: Uma linhagem `settled` ou `declined` que reaparece registra a ocorrência e não muda de status.

**Convergência e veredito**

- R23: Uma rodada é limpa quando, **depois** do matching e da reavaliação, não há linhagem blocker/major em `open`, `reopened`, `contest_pending` ou `escalated`, e nenhum revisor está `not_run`.
- R24: `converged: true` exige `egm.clean_rounds` (padrão 2) rodadas limpas consecutivas. `capped: true` vale quando a rodada `egm.max_rounds` (padrão 5) termina sem convergir. Rodadas depois do teto exigem `--past-cap` com confirmação humana (R38) e só chegam, no máximo, a `PASS_PARTIAL`.
- R25: Vereditos:
  - `IN_PROGRESS`: o loop ainda não convergiu nem atingiu o teto.
  - `PASS`: convergiu com cobertura completa (9 revisores `completed` em todas as rodadas contadas), sem `--force-settle`, sem `--past-cap` e sem `--restart`.
  - `PASS_PARTIAL`: convergiu com algum revisor `unavailable` ou `skipped`, com `--reviewers` restrito, `--force-settle`, `--past-cap` ou `--restart`.
  - `FAIL`: atingiu o teto (`capped`) sem convergir e sem `--past-cap`.

  Uma rodada com cobertura menor que a da rodada 1 é recusada, salvo com `--narrow --reason` e confirmação humana. Começar estreito já na rodada 1 é permitido, mas o resultado é `PASS_PARTIAL`.
- R26: O `loop next` é recusado quando nem o `doc_sha256` nem nenhum sha das `sources` mudou desde a rodada anterior **e** há linhagens blocker/major em `open` ou `reopened`. A mensagem é `documento e fontes não mudaram desde a rodada N; há findings bloqueantes abertos`.
- R27: (removido na rodada 2; churn foi movido para a Fase C.)
- R28: O tb conta as execuções de agente realizadas no loop (revisores, retries e árbitros) e para em `egm.max_agent_runs` (padrão 120). Os revisores pendentes viram `not_run` com `reason: budget`. Antes de cada rodada, o pior caso é `revisores × 2 + contestações pendentes × 2`. `--dry-run` imprime esse pior caso e o restante do orçamento. Uma rodada cujo pior caso excede o restante é recusada, salvo com `--allow-partial-budget`.

**Resolve e árbitro**

- R29: `tb egm resolve <doc>`. Repetir uma transição para o status atual é no-op com sucesso. Qualquer outra transição inválida é erro e não altera nada.

  | Flag | De | Para | Restrição |
  |------|----|------|-----------|
  | `--fix F-…` | open, reopened, escalated | fixed | o `doc_sha256` ou o sha de alguma fonte citada pela linhagem mudou desde a `origin_round` (caso contrário, recusa); a reavaliação do R20 confirma ou reabre |
  | `--contest F-x --evidence path:line` | open, reopened | contest_pending | `contests == 0`; a evidência está nas `sources` da linhagem ou na própria `evidence` do finding, num arquivo versionado com o mesmo blob em `head_commit` da `origin_round` e no `HEAD`, sem mudanças no worktree; persona ≠ comprehension |
  | `--escalate F-x --note "…"` | open, reopened | escalated | `contests == 0` |
  | `--decline F-…` | open, reopened | declined | só `nit`; `contests == 0` |
  | `--force-settle F-x --reason "…"` | open, reopened, contest_pending, escalated | settled | confirmação humana (R38); `forced: true`; veredito máximo `PASS_PARTIAL` |
  | `--dup-of F-a F-b` | F-a ∈ open, reopened, com `contests == 0` | F-a fundido em F-b | F-b ∈ open, reopened ou fixed; `overlap(scope) ≥ egm.match.overlap`; F-b herda a maior severidade e o status mais restritivo (R30) |

- R30: Ordem de restrição de status, da mais restritiva para a menos: `escalated` > `contest_pending` > `reopened` > `open` > `fixed`.
- R31: No `loop next`, antes da rodada, cada linhagem `contest_pending` vai a um árbitro fresco. O árbitro recebe o finding, o trecho do `scope` no snapshot e a evidência (±20 linhas do blob no `HEAD`, redigidas e já checadas pelo R17a), tudo nos blocos do R9. A saída é validada contra `internal/egm/schema/arbiter.schema.json` (`{"nonce","verdict":"withdraw|uphold","rationale"}`), com retry e `not_run` como no R16. Resultado:
  - `withdraw` → `settled` (`by: arbiter`);
  - `uphold` → `open`, com `contests = 1`;
  - `not_run`, ou sem agente → continua `contest_pending`.

  Depois de `uphold`, a linhagem só sai por `--fix` (com reavaliação), por `still_holds: false` do revisor dono ou por `--force-settle`.
- R32: Uma linhagem blocker/major `escalated` só sai por `--fix` ou `--force-settle`.

**Integração**

- R33: Comandos:
  - `tb egm review <doc>` cria a rodada 1. Se o estado já existe, dá erro e sugere `loop next` ou `--restart`.
  - `--restart` abre uma nova série de rodadas **sobre** o estado existente. Mantém linhagens, `contests`, histórico e contagem total de rodadas. Exige confirmação humana. O veredito máximo passa a ser `PASS_PARTIAL`, e o relatório mostra `restarted: N`.
  - Se `<doc>` não tem estado, mas `git diff --find-renames <ref de confiança>..HEAD` mostra que ele foi renomeado a partir de um path que tem estado, o `review` recusa e exige `--adopt <path-antigo>`, que transfere o estado.
  - Também existem `loop next <doc>`, `resolve <doc>` e `status <doc>`.

  Todos aceitam `--json`. As MCP tools `tb_egm_review`, `tb_egm_loop_next`, `tb_egm_resolve` e `tb_egm_status` ficam em `internal/registry/parity.go`. Flags que exigem confirmação humana são recusadas pelo MCP.
- R34: Códigos de saída:
  - `review`/`loop next`: 0 quando a rodada completa sem `FAIL`; 1 em `FAIL`; 2 em erro de uso ou execução.
  - `resolve`: 0 em sucesso ou no-op; 2 em transição inválida.
  - `status`:
    - `PASS`: 0;
    - `PASS_PARTIAL`: 0, ou 1 com `--strict` ou `egm.require_full_coverage: true`;
    - `IN_PROGRESS`: 3;
    - `FAIL`: 1;
    - registros com HMAC inválido: 4.
- R35: `tb stack` ganha:
  - op `egm` (`tb stack egm --egm-doc <doc>`), que roda `status`;
  - flag `--strict`;
  - campos `EgmDoc` e `Strict` em `stack.Options`;
  - argumentos `egm_doc` e `strict` no schema da tool `tb_stack` em `parity.go`, com a descrição do array `ops` atualizada para incluir `egm`;
  - campo aditivo `warn` em `StepResult` e em `Result` (OR dos passos).

  Mapeamento:
  - `PASS` → `ok: true`;
  - `PASS_PARTIAL` → `ok: true, warn: true`, ou `ok: false` com `--strict` ou `require_full_coverage`;
  - `IN_PROGRESS`, `FAIL` e HMAC inválido → `ok: false`.

  Com `warn`, o stack lista as linhagens `forced` e os `restarted`. O código de saída com `warn` é 0.
- R36: `review.Finding`, `tb review` e `tb drift` não mudam.

**Fronteira de confiança e auditoria**

- R37: As chaves `egm.agent_cmd`, `vendor`, `tools_disabled`, `forbidden_flags`, `output`, `env`, `home`, `reviewers`, `clean_rounds`, `max_rounds`, `match`, `require_full_coverage` e `max_agent_runs` são **sensíveis**. Elas são lidas, nesta ordem:
  1. de `$XDG_CONFIG_HOME/tb/egm.yaml`, a config de usuário fora do repositório;
  2. de `.toolbelt/config.yaml` no **ref de confiança**, via `git show`, como o `guard` faz com `guard.yaml`. O ref de confiança é `egm.trust_ref`, que vale `origin/HEAD` e, se não existir, `main`.

  Se o valor no worktree divergir, ele é ignorado com o aviso `config_untrusted`. `.toolbelt/config.yaml` entra nos `protectedPaths` padrão do `guard`. Limite declarado: um processo do mesmo usuário que edite a config de usuário ou faça commit no ref de confiança está fora do alcance desta fase. Contra isso valem o CI e o review humano do commit.
- R38: Cada registro de rodada e cada transição levam um HMAC-SHA256 encadeado (`prev_hmac`), com chave em `$XDG_STATE_HOME/tb/egm.key` (`0600`, criada na primeira execução).
  - O `status` e o `tb stack egm` recalculam o veredito a partir dos registros verificados, não do `state.json` agregado.
  - Os registros também vão, append-only, para `.toolbelt/egm/audit.jsonl`.
  - **Confirmação humana:** stdin e stdout precisam ser TTY, e o usuário digita o ID da linhagem (ou `restart`, `narrow` ou `past-cap`). Sem TTY, a flag é recusada. A confirmação é registrada no audit com `by: human`.
  - **Limite declarado:** quem tem shell como o mesmo usuário pode ler a chave ou forjar um TTY. O objetivo é deixar a adulteração **evidente** e cara, não impossível. Tokens de capacidade (ADR-005 §4) e um usuário separado ficam para depois.

### Configuração

```yaml
# Chaves sensíveis (R37): lidas de $XDG_CONFIG_HOME/tb/egm.yaml ou do ref de confiança.
egm:
  agent_cmd: []              # vazio → deterministic
  vendor: claude             # claude | custom
  output: claude-json        # claude-json | raw (padrão do custom: raw)
  tools_disabled: false      # obrigatório true com vendor: custom
  forbidden_flags: []
  env:
    pass: []                 # ex.: [ANTHROPIC_API_KEY], somado a env.pass global
  home:
    copy: []                 # só se o R0 escolher cópia
    read_only: []            # ex.: ~/.nvm/versions/node/v22.11.0
  reviewers: {}              # <nome>: {agent_cmd: [...]}
  match: {sim: 0.3, overlap: 0.5}   # 0 < x ≤ 1
  require_full_coverage: false
  max_rounds: 5              # ≥ 1
  clean_rounds: 2            # 1 ≤ clean_rounds ≤ max_rounds
  max_agent_runs: 120        # ≥ 1
# Chaves comuns (podem vir do worktree):
  trust_ref: origin/HEAD
  detail_headings: ["## Detailed Implementation Plan", "## Plano de Implementação Detalhado", "## Implementation Plan"]
  max_parallel: 4            # ≥ 1
  timeout_per_reviewer: 5m   # time.ParseDuration, > 0
  max_source_bytes: 65536
  max_packet_bytes: 409600
  max_stdout_bytes: 1048576
```

## Modelo de ameaças

| Ameaça | Mitigação |
|--------|-----------|
| O documento (escrito pelo elephant) instrui os revisores | R17a: scan de padrões dirigidos ao revisor; R17b: atestação literal obrigatória; R9: blocos de dado. Risco residual: uma injeção sutil que o scan não pega e que o modelo obedece em todos os revisores, ainda produzindo atestações válidas. |
| Config adulterada (agente falso, `clean_rounds: 1`) | R37 (chaves sensíveis vêm da fonte de confiança; guard protege `.toolbelt/config.yaml`) e R1/R21 (`argv_sha256`, `binary_sha256` por rodada) |
| `state.json` ou relatório editados à mão | R38 (HMAC encadeado; veredito recalculado dos registros; exit 4) |
| Loteria (`--fix` sem mudança, repetir até esquecer) | R29 (`--fix` exige mudança no documento ou nas fontes), R20 (reavaliação pelo dono; omissão invalida), R26 |
| Re-sortear o árbitro, `--restart` ou `git mv` | R29/R31 (uma contestação), R33 (`--restart` mantém o histórico e rebaixa; renomeação exige `--adopt`) |
| Injeção no árbitro via evidência versionada | R7 (exclui `testdata/` e personas), R29 (evidência precisa ser fonte da linhagem), R17a (scan do excerto), R31 (blocos de dado) |
| `--dup-of` de blocker para nit e depois `--decline` | R29 (herda a maior severidade e o status mais restritivo; sem contestação prévia) |
| Escape hatches usados pelo próprio agente | R38 (confirmação humana por TTY, recusada no MCP), R25 (rebaixa para `PASS_PARTIAL`), R35 (stack lista forçados e restarted) |
| Gate de CI sem revisão LLM | R25 (cobertura parcial), R34/R35 (`--strict` / `require_full_coverage`, protegidos pelo R37) |
| Goldfish lê `/proc/<pid>/environ`, outros pacotes ou a credencial | R5 (sem nenhuma ferramenta), R2 (workdir RO e `TMPDIR` privado) |
| Contaminação pelo `CLAUDE.md` global | R4 (HOME descartável), R0 (verifica) |
| Segredo enviado ao provedor | R7 (deny-list e só versionados), R8, `execenv.Filter` |
| Credencial vazada ou rotacionada | R0 (prefere env ou config dir; testa refresh), R4 |
| Saída falsa no stderr ou JSON truncado | R2 (stdout separado; overflow invalida) |

## Requisitos não funcionais (NFRs)

- **Determinismo:** no modo `deterministic`, as mesmas entradas geram o mesmo `report.json` byte a byte (ordenação estável, sem timestamps no corpo comparado).
- **Custo:** cerca de 9 execuções por rodada, limitadas pelo R28.
- **Segurança:** invariantes de `tb mem list`.

## Alternativas

Ver o [ADR-014](../adr/014-egm-nativo.md#alternativas-consideradas).

## Questões em aberto

- A forma de autenticação, os flags sem ferramentas e os diretórios de leitura do perfil `claude`, além dos limiares de matching, saem do R0.

## Plano de Implementação Detalhado

| Arquivo | Ação | Motivo |
|---------|------|--------|
| `internal/sandbox/sandbox.go`, `internal/sandbox/landlock_linux.go`, `internal/sandbox/landlock_other.go` | Modify | `ReadOnlyWorkDir`, `PrivateTmp`, `Home`, `Stdin`, `Stdout`/`MaxStdoutBytes` e regra de arquivo sem `ReadDir` (R2) |
| `bench/egm-spike/` | Create | Spike (R0) |
| `internal/egm/vendor.go` | Create | Perfis `claude` e `custom`, autenticação, flags, envelope e recusa de bypass (R4, R5, R15) |
| `internal/egm/run.go` | Create | Goldfish via `sandbox.New`, Registry e subreaper, timeout, retry (R3, R16) |
| `internal/egm/packet.go`, `internal/egm/sources.go` | Create | Snapshot, `resolveSource`, deny-list, limites, prompt e persistência (R1, R7, R9) |
| `internal/graph/index_docs.go` | Modify | Exportar `CollectMDLinks`, `ResolveDocHref`, `NormalizeRelPath`, `CollectPathLikes`, `CollectBacktickSpans`, `LooksLikePath` e `LooksLikeSymbol`; novas extensões (R7, R13) |
| `internal/mem/redact.go` | Modify | Padrões novos e preservação de linhas (R8) |
| `internal/egm/personas/*.md`, `internal/egm/standards/*.md` | Create | 8 personas e `arbiter` (com `max_severity`); standards `feature-spec`, `implementation-readiness` e `zoom-out` (R10, R11, R31) |
| `internal/egm/deterministic/{refs,sections,planfiles}.go` | Create | Checkers offline e parsing do plano (R13) |
| `internal/egm/injection.go`, `internal/egm/injection_patterns.txt` | Create | Scan anti-injeção (R17a) |
| `internal/egm/attest.go` | Create | Validação da atestação (R17b) |
| `internal/egm/finding.go`, `internal/egm/schema/finding.schema.json`, `internal/egm/schema/arbiter.schema.json` | Create | Tipos, schemas, teto, nonce e evidência (R11, R15–R17) |
| `internal/egm/match.go` | Create | normalize, sim, overlap, fusão, dono, mapeamento por diff e linhagem (R18–R20) |
| `internal/egm/state.go` | Create | Registros, transições, convergência, veredito, cobertura e orçamento (R21–R26, R28) |
| `internal/egm/trust.go` | Create | Fonte de confiança, `config_untrusted`, HMAC encadeado, audit, confirmação por TTY (R37, R38) |
| `internal/egm/resolve.go` | Create | Tabela de transições e árbitro (R29–R32) |
| `internal/egm/review.go` | Create | Orquestração da rodada, `--restart` e `--adopt` (R6, R10, R12, R33) |
| `internal/config/config.go`, `internal/config/exclude.go` | Modify | `EgmConfig` com Validate; `.toolbelt/egm/` no exclude (R21) |
| `internal/guard/guard.go` | Modify | `.toolbelt/config.yaml` nos protected paths padrão (R37) |
| `cmd/tb/egm.go` | Create | Grupo cobra (R33, R34) |
| `internal/stack/stack.go`, `cmd/tb/stack.go` | Modify | Op `egm`, `EgmDoc`, `Strict`, `warn` (R35) |
| `internal/registry/parity.go` | Modify | Tools `tb_egm_*`; schema da `tb_stack` (`egm_doc`, `strict`, descrição de `ops`) (R33, R35) |
| `internal/bench/fake_agent.go`, `cmd/tb/fake_agent.go` | Modify | Ler o prompt do stdin quando `--prompt` estiver vazio e extrair `NONCE:`, `PRIOR:` e os blocos de dado; modo por `egm_mode` no `--config` (`findings`, `invalid`, `no-nonce`, `over-severity`, `omit-prior`, `bad-attest`, `stderr-json`, `overflow`, `sleep`, `probe-fs`, `escape`) |
| `docs/cli.md`, `docs/arquitetura.md` §8.5 (+ `docs/en/`) | Modify | Documentar `tb egm`; trocar o exemplo fictício `tb review pr:42` |
| `CHANGELOG.md` | Modify | Entrada não lançada |

### Plano de verificação

- **R0:** script em `bench/egm-spike/`. Resultado em `docs/campo/dogfood.md` (PT e EN). Corpus de paráfrases em `internal/egm/testdata/paraphrases.json`.
- **Sandbox:** testes para:
  - workdir RO (escrita falha);
  - `PrivateTmp` (o `/tmp` compartilhado fica ilegível);
  - `Home` (o HOME real fica ilegível e o `Home` prevalece sobre `Config.Env`);
  - stdout separado e overflow;
  - regra de arquivo sem EINVAL;
  - fake-agent `probe-fs` e `escape`.

  Skip só é legítimo sem Landlock.
- **Execução:** cada modo do fake-agent, recusa de bypass, `custom` sem `tools_disabled`.
- **Anti-trapaça (tabelas):**
  - `--fix` sem mudança no documento ou nas fontes é recusado;
  - a segunda `--contest` e a evidência fora das fontes ou alterada são recusadas;
  - evidência com injeção é recusada;
  - `--dup-of` blocker→nit seguido de `--decline` é recusado;
  - documento com injeção gera `major` do `injection-scan`;
  - **esta spec não dispara o scan**;
  - atestação inválida dá `not_run`;
  - `clean_rounds` no worktree gera `config_untrusted`;
  - `state.json` editado dá exit 4;
  - escape hatch sem TTY é recusado;
  - `--restart` mantém `contests` e rebaixa;
  - `git mv` exige `--adopt`;
  - estreitamento de cobertura é recusado.
- **Match:** corpus do R0, linhas deslocadas, heading renomeado, reavaliação pelo dono (as duas instâncias de `style`) e fechamento por rerun.
- **Estado:**
  - transições do R29 e idempotência;
  - os quatro vereditos e `capped`/`--past-cap`;
  - R26 com fonte alterada;
  - árbitro `withdraw`, `uphold` e `not_run`;
  - orçamento.
- **Golden:** `report.json` determinístico desta spec, incluindo o parsing do seu próprio plano (R13).
- **Regressão:**
  - JSON de `tb review` e `tb drift` igual ao golden atual (R36);
  - saída do `tb mem` (R8);
  - `guard` com o novo protected path;
  - `go test ./...` e `TestDocsEnglishMirrorComplete`.

*Elaborado com base no padrão EGM*
