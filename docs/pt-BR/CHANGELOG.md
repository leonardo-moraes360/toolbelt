[🇺🇸 English](../../CHANGELOG.md) | 🇧🇷 Português

# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Não Lançado]

### Alterado
- `tb egm draft`: um rascunho novo e o `--update` agora usam os mesmos globs para o alvo (`egm.doc_globs` mais os globs extras do repositório; o `bootstrap` divide o helper), e `--body` com valor parecido com segredo é recusado (`draft_secret`) antes de escrever qualquer coisa. A R10 da spec agora exclui o diretório `drafts/` de propostas do `--update`
- Config do `tb verify` (R0): com `-c`, nada muda (só o arquivo do `-c` e os defaults; a config do repositório é ignorada). Sem `-c`, o `.toolbelt/config.yaml` do repositório é lido como commitado no ref base (sobre a config global e os defaults), nunca da árvore de trabalho: edições não commitadas são ignoradas, com um aviso que cita as chaves (`ignored .toolbelt/config.yaml keys that differ from the base ref: …`, comparando o arquivo na raiz do repositório) — uma mudança de comportamento para quem editava chaves de `verify:` sem commitar. `env.allow`/`env.pass` também vêm do arquivo do base, então uma mudança não consegue passar segredos aos próprios testes. Sem arquivo no base, valem só a config global e os defaults e o relatório traz `config_source: "none"`; nos demais casos `config_source` é omitido. Um arquivo do base commitado como symlink não é seguido (`config_source: "none"` mais um aviso); um arquivo do base malformado aborta o verify sem `-c`. Uma config global malformada agora aborta o verify (antes era ignorada em silêncio). Flags e argumentos MCP continuam sobrescrevendo. O guard reporta o novo achado de erro `verify_config_changed` (aborta o verify, lista as chaves alteradas) quando uma mudança edita a seção `verify:` (uma config symlink é comparada pelo texto do alvo; qualquer mudança lista `verify`), e o achado de aviso `verify_config_unreadable` quando o arquivo do base está malformado (o guard e execuções com `-c` seguem funcionando); as outras seções seguem livres. O CI precisa fixar o base (o merge base do PR); o verify avisa quando o SHA do base é igual ao do head
- `tb verify run --check`, `tb stack verify --check` e o argumento `checks` do MCP: um nome de check inválido (`[a-z0-9-]{1,32}`) ou duplicado é erro de uso (exit 2; erro de ferramenta no MCP)
- `tb verify run --ruler` com um nome desconhecido (ou `visual` combinada com outra régua) agora falha antes de qualquer coisa rodar, com exit 2 (novo código de erro de uso), em vez de rodar `tests` em silêncio
- O `tb init` numa config nova grava um script `typecheck`/`type-check` do Node como `verify.typecheck_cmd` (antes era `build_cmd`) e deixa de usar mypy como fallback de lint do Python; configs existentes não são reescritas
- Kernels de embedding do amd64 no nível AVX2 (ADR-019, aceito; indexação 3,2× mais rápida num runner de CI, recall idêntico, binário +1,2%): o ggml-cpu é compilado com AVX, AVX2, FMA, F16C e BMI2 (`TB_X86_KERNELS=v2` mantém o piso antigo); o resto do `tb` mantém o piso x86-64-v2. O `tb __graph-embed` verifica a CPU primeiro e recusa com o motivo novo `embed_cpu_unsupported`, então o código nativo nunca roda abaixo desse nível. Novas etapas do gate: `scripts/avx-audit.py` (código VEX/EVEX/BMI2 só em símbolos que só a libggml-cpu.a define), um teste de que só o filho alcança `native.Open`, e uma execução sob QEMU Nehalem de outros comandos e da recusa
- Shim de embedding: os textos de uma requisição são agrupados em avaliações nativas compartilhadas (`tbe_embed_batch`, `n_seq_max` 64, KV unificado, orçamento de 512 tokens) em vez de uma chamada por texto; os vetores não mudam (cosseno ≥ 0,999). No build x86-64-v2: indexação cerca de 4% mais rápida, textos curtos cerca de 21% mais rápidos

### Adicionado
- Régua `coverage` do `tb verify`, fatia 2 ([spec](specs/verify-rulers.md) R4): `--ruler coverage`, combinável (`--ruler tests,coverage`; `visual` continua sozinha), também no `tb stack verify` e no MCP. Flags `--coverage-cmd`, `--coverage-pattern`, `--min-coverage` (MCP `coverage_cmd`, `coverage_pattern`, `min_coverage`; config `verify.coverage_cmd`, `verify.coverage_pattern`). Padrão Go: `go test -coverprofile`, blocos deduplicados, total ponderado por statements; o modo genérico usa a última ocorrência de uma regexp com um grupo (0–100), e um `coverage_cmd` sem `coverage_pattern` falha com motivo próprio. O threshold é `coverage.min_percent` do `.toolbelt/thresholds.yaml` **só no ref base** (`version: 1`; arquivo malformado ou symlink aborta o verify; `--min-coverage` só pode subir, `threshold_source` é `base` ou `flag`). Sem threshold é `SKIPPED` / `not_applicable` (não rebaixa um `PASS` combinado); threshold sem comando de cobertura, ou valor abaixo dele, é `FAIL`. O JSON ganha `coverage {percent, threshold, threshold_source, cmd, metric}`, `metrics.coverage_percent` e, com várias réguas, `{verdict, rulers: {tests, coverage}}`; overrides por flag ou MCP aparecem em `overrides`
- `tb egm` Fase C, fatias C4 e C5 (`docs/specs/egm-phase-c.md`): drift no nível do símbolo. Uma fonte Go pode ser citada como `path.go#Name` ou `path.go#Recv.Name` (link ou trecho entre crases; âncoras `#L10` continuam no nível do arquivo); o novo extrator puro `graph.GoSymbols` e o checker `refs` conferem o símbolo no snapshot (ausente: `major`; ambíguo, `init`, `_` ou duplicado: `symbol_ambiguous`; arquivo que não faz parse: nit), sem grafo e sem LLM, e as rodadas guardam `symbols`. O recibo versão 3 (`egm.receipt.write_v3: true`, só do usuário, implica o `commit` da v2) acrescenta `symbols` (sha256 dos bytes crus de cada símbolo); os leitores aceitam de 1 a 3 e o CI recalcula os hashes de símbolo no head. O `drift-report` (`egm-source-drift/0.2`) compara uma fonte citada só por símbolos pelos símbolos dela: `symbol_changed` e `symbol_missing` são drift (exit 1), enquanto uma mudança em outro ponto do arquivo é `outside_symbols` e o documento continua `fresh`; o `ci_source_drift` acompanha. Comportamento novo: um trecho entre crases terminando em `.go#Name` agora é candidato a fonte, então um documento que já tinha um ganha uma fonte (`sources_differ` uma vez); nenhum existe neste repositório. Faixas de linha do grafo e IDs de método com receptor (C6) ficam adiados para depois da 1.0
- `tb egm brainstorm` e `tb egm draft` (Fase C, slices C1–C3, [spec](specs/egm-phase-c.md)). `brainstorm --brief rd.md [--from ...] [--out f]` roda um goldfish isolado (persona embutida `brainstorm`) e escreve um concept brief (Problem, Ideas, Recommended direction, Open questions) que nunca é gated nem revisado: por padrão em `$XDG_STATE_HOME/tb/egm/<repo-id>/briefs/` (modo 0600), ou em `--out`, recusado com `brainstorm_gated` quando casa com `egm.doc_globs`. `draft <doc> (--body <arquivo> | --engine llm) [--brief] [--from ...]` escreve uma spec ou ADR gated nova, com linha de status de gerado, toda seção obrigatória (TODO quando falta) e `## Sources` dos arquivos de `--from`; `--body` é determinístico (sem agente), `--engine llm` roda a persona `draft`. O corpo é limpo, varrido contra injeção e só pode citar as fontes aceitas (`draft_sources`); o brief não é citado. `draft --update` nunca edita um documento: escreve uma proposta em `<state>/egm/<repo-id>/<slug>/drafts/` e imprime um diff. Nenhum dos dois escreve estado de revisão, rodada ou recibo, então um rascunho só chega a `PASS` por `tb egm review`. Briefs com valor parecido com segredo ou texto de injeção são recusados antes de qualquer agente (`brief_secret`, `brief_injection`). Novas chaves de limite `egm.brainstorm.max_bytes` (16 KiB) e `egm.draft.max_bytes` (32 KiB); a config do repositório só pode diminuí-las
- Réguas adicionais do `tb verify`, fatia 1 ([spec](specs/verify-rulers.md) R0–R3, R5, R6): a régua `tests` ganha uma etapa de gate `typecheck` (`verify.typecheck_cmd`, `--typecheck-cmd`, `--no-typecheck`) e checks nomeados (`verify.checks: [{name, cmd}]`, `--check name=cmd` e `--no-check name`, ambos repetíveis, `--no-checks`). Ordem das etapas de gate: build → typecheck → lint → checks → testes visíveis → ocultos; cada gate roda uma vez por execução e sua falha é `ESCALATE`; um executável de check ausente sai com 127. `--ruler` aceita uma lista separada por vírgulas (`tests`, `visual`; `visual` não pode ser combinada). Chaves JSON novas, todas `omitempty`: `typecheck_ran`, `typecheck_passed`, `checks` (`[{name, passed, reason}]`), `config_source` (`none`, só sem arquivo no base), `warnings`, `overrides` (chaves do verify, em ordem, que uma flag ou argumento MCP mudou em relação à config confiável, mais toggles de skip que desligaram uma etapa configurada, ex. `test_cmd`, `no_typecheck`, `skip_checks:govulncheck`, `no_checks`; o modo texto, também no `tb stack`, imprime-os junto com os avisos no stderr), `skip_kind`. O `tb stack verify` ganha as mesmas flags mais `--ruler` (`visual` recusada); os `tb_verify_run` e `tb_stack` do MCP ganham `typecheck_cmd`, `no_typecheck`, `checks`, `skip_checks`, `no_checks`, e o `tb_stack` ganha `ruler`
- Detecção de typecheck e checks no `tb init`: um script `typecheck`/`type-check` do Node, ou `tsconfig.json` com um `tsc` local (`npm exec -- tsc --noEmit`, `pnpm exec tsc --noEmit`, `yarn tsc --noEmit`, `bunx tsc --noEmit`), e um mypy/pyright configurado que resolve no PATH ou em `.venv/bin` viram `verify.typecheck_cmd` (`uv run …` com `uv.lock`, senão o nome puro no PATH, senão `.venv/bin/mypy .` / `.venv/bin/pyright`). `tb init --write-checks` adiciona `govulncheck` (Go, quando está no PATH) e `clippy` (Rust, quando `cargo clippy -V` funciona) a `verify.checks` de uma config nova; sem a flag, as sugestões vão para uma linha de log e para `suggested_checks` no `--json`. Os checks só são sondados (o que roda `cargo clippy -V`) quando o init cria a config ou com `--write-checks`
- Índice vetorial V2 (`docs/specs/vector-index.md`; deixa o status experimental): o `tb_graph_search` do MCP ganha `mode` (`lexical`|`semantic`|`hybrid`, padrão `lexical`; o mesmo JSON da CLI mais um array `warnings`) e o `tb_graph_index` ganha `vectors` (booleano, padrão `false`; o payload ganha as estatísticas `vectors`); sem eles a saída não muda, as consultas pelo MCP nunca reindexam os vetores e os erros usam os mesmos motivos, com os avisos anexados. O `tb mcp` mantém um filho de embedding aquecido por servidor (reaproveitado entre consultas, reiniciado quando a config muda ou o filho quebra, morto depois de 5 minutos ocioso e na saída). CLI e MCP compartilham o `internal/vecsearch`. Avaliação de recall: `internal/graph/eval/semantic.yaml` (44 consultas por intenção sem tokens de identificadores ou nomes de arquivo) e `TestSemanticRecall` (condicionado a `TB_EMBED_MODEL` e `TB_EMBED_MODEL_SHA256`; exige `-tags embednative` ou `TB_EMBED_EXE`). Com all-MiniLM-L6-v2 Q8_0 e llama.cpp b12600: recall@10 lexical 0,273, semantic 0,670, hybrid 0,568
- Índice vetorial V1 (`docs/specs/vector-index.md`): `tb graph index --vectors` (também no `graph rebuild`) cria ou atualiza o `.toolbelt/graph.vec` (0600, atômico), refazendo o embedding só dos chunks cujo sha256 do texto mudou; o JSON ganha `vectors` (`path`, `files`, `units`, `embedded`, `reused`, `dropped`, `denied`, `truncated`, `rebuilt`) e o texto uma linha `vectors: …`. `tb graph search --mode lexical|semantic|hybrid` (padrão `lexical`, saída inalterada): `semantic` ranqueia por cosseno sobre chunks agrupados por nó de arquivo, `hybrid` funde léxico e semântico com RRF (k = 60, empates por ID do nó); o JSON deles é `{"stale", "mode", "stale_vectors", "nodes"}` com `lines`, `score_semantic`, `rank_lexical`, `rank_semantic`, `score_fused`, e o texto acrescenta `:<lines>` ao path; `--mode` nunca reindexa. As chaves `graph.vector.*` e `graph.embed.*` (`enabled`, `model`, `model_sha256`, prefixos, `threads`, `timeout`, `allow_unsandboxed`, `max_chunks`, formato dos chunks) vêm só da config do usuário (valores do repositório ignorados, `config_untrusted`). O filho `tb __graph-embed` roda em sandbox (sem sockets, workdir vazio e só leitura, tmp privado, `HOME` vazio, env mínimo); arquivos da deny list nunca viram embedding e os arquivos são redigidos por inteiro antes dos chunks. Motivos novos: `vectors_disabled`, `vectors_stale`, `vectors_invalid`, `vectors_too_many`, `model_path`, `model_mismatch`, `model_unsupported`, `embed_unsandboxed`, `embed_protocol`, `embed_crashed`, `embed_timeout`, `config_untrusted`. `mode`/`vectors` no MCP e a avaliação de recall ficam para a V2
- Índice vetorial V0 (ADR-018, `docs/specs/vector-index.md`): todo binário do release é compilado com cgo e um subconjunto vendorizado em allowlist do llama.cpp (`third_party/llama.cpp`, tag b12600, `MANIFEST` com os hashes do tarball e do subconjunto, `scripts/vendor-llama.sh --check`), só CPU no piso x86-64-v2/armv8.0-a, com build cruzado por um zig fixado (glibc 2.28, macOS 11). `tb version` (e o campo `embed` do `--json`) cita o motor; o comando oculto `tb __graph-embed --smoke <modelo> --sha256 <hex>` carrega um modelo GGUF fixado (path absoluto, sem symlinks, do próprio usuário, sha256 conferido) e verifica que uma paráfrase pontua acima de um texto sem relação. Um build sem `-tags embednative` não tem código nativo e responde `embed_unavailable`. Cada archive leva a licença e o manifesto do llama.cpp em `THIRD_PARTY_LICENSES/`, e cada SBOM lista llama.cpp e ggml no commit fixado (`scripts/sbom.sh`). Os builds do release usam `netgo,osusergo`, como os releases anteriores em Go puro; o darwin linka por stubs `.tbd` gerados para as bibliotecas de sistema que o próprio Go importa (`scripts/darwin-stubs.sh`). O release é publicado como draft e só fica público depois de passar a matriz de smoke nativa em seis runners, o `rockylinux:8` e o `qemu -cpu Nehalem`. O novo gate `native` da CI roda o shim com gcc/clang `-Werror`, clang-format, clang-tidy e cppcheck, os testes de embed com o modelo de smoke, ASan/UBSan, libFuzzer (5 minutos por PR, 1 hora à noite), uma checagem por strace de que inicializadores estáticos não fazem I/O, checagens de hardening e do piso de glibc nos artefatos Linux, CodeQL `c-cpp` e uma checagem diária de advisories do llama.cpp; só `internal/embed/native` pode importar `"C"` (um teste: o depguard não enxerga o pseudo-pacote do cgo). Nenhum comando do grafo usa embeddings ainda
- Rascunho da spec do índice vetorial (`docs/specs/vector-index.md`) e ADR-018 (proposta): o release passa a usar cgo para o llama.cpp vendorizado (só CPU, com piso x86-64-v2/armv8.0, builds cruzados com zig e fallbacks nomeados, glibc ≥ 2.28, matriz de smoke nativa com os seis alvos) sob um gate de qualidade de C/C++ `native` (lint e análise estática do shim, hardening por plataforma verificado nos artefatos, builds com sanitizers, libFuzzer, um subconjunto do vendor em allowlist com verificação de manifesto e de advisories); os embeddings rodam num filho `tb __graph-embed` em sandbox e sem rede (novo bloqueio de sockets por seccomp) com um modelo GGUF fixado pelo usuário; o `tb graph search --mode lexical|semantic|hybrid` (RRF) continua opt-in, com uma avaliação de recall@10 decidindo o status experimental

### Corrigido
- `TestLLMFlowEscape` intermitente (e o fake em shell por trás de `TestRunnerEscapeAttributed` / `TestRunnerUnattributedKillsRunning`): os modos `escape` dos agentes fake rodavam o `setsid` num filho em background e saíam sem esperar por ele, então sob carga o agente podia sair com o filho ainda no seu grupo de processos; o kill do grupo o levava em silêncio e o revisor voltava `completed` em vez de `not_run:escaped_process`. O `__fake-agent-egm` agora inicia o fugitivo com `setsid(2)` antes do exec (o `exec.Cmd.Start` retorna depois do exec), e o fake em shell espera um marcador que o fugitivo escreve depois do `setsid`. Só testes; a detecção de fuga não muda

### Segurança
- `tb verify`: ler `verify.*` e `env.*` do ref base fecha a brecha em que a mudança sob teste podia reescrever o próprio gate (ex. `test_cmd: true` no `.toolbelt/config.yaml`) ou passar segredos aos próprios testes (`env.pass`)
- `internal/sandbox`: novo filtro seccomp `DenyNetwork` que bloqueia `socket`, `socketpair` e `io_uring` (usado pelo filho `tb __graph-embed`)
- `internal/secrets`: deny list e redação compartilhadas, usadas pelo índice vetorial antes dos chunks

## [0.0.10] - 2026-11-21

### Adicionado
- Fase B3 do `tb egm` (`docs/specs/egm-drift-bootstrap.md`): o `drift-report --engine llm` roda um goldfish de drift isolado (persona nova `drift`) por documento `drifted`, até `egm.drift.max_docs` (padrão 5, chave de limite), sobre o documento redigido, cada fonte mudada e, quando o `commit` consultivo do recibo é utilizável, o diff unificado dela (`diff --no-index -U3` num diretório temporário privado; um lado com cara de segredo deixa a fonte de fora); os findings saem em cada documento, nunca são guardados e não mudam o exit code. O `bootstrap --engine llm` roda um goldfish de bootstrap (persona nova `bootstrap`) sobre os arquivos aceitos e põe o corpo Markdown dele no lugar das seções TODO, depois de tirar comentários HTML, títulos de nível 1, linhas `Status:` iniciais e `## Sources`; uma injeção no corpo ou numa fonte (`bootstrap_injection`), uma citação de outro arquivo (`bootstrap_sources`) ou um corpo vazio, grande demais (`egm.bootstrap.max_bytes`, padrão 32 KiB) ou fora de UTF-8 (`bootstrap_invalid`) não escreve nada. Os dois recusam um perfil cujo uso de tools não dá para verificar (`tools_unverifiable`) ou sem `egm.agent_cmd` (`agent_unavailable`). Os recibos podem ser da versão 2, com `commit` (o HEAD na escrita, consultivo); os leitores aceitam 1 e 2, e o tb só escreve a 2 com a chave só do usuário `egm.receipt.write_v2: true`
- `tb egm bootstrap <doc-novo> --from <caminho>...` (Fase B2, `docs/specs/egm-drift-bootstrap.md`): escreve um documento novo sob gate (sem nunca sobrescrever, num diretório de verdade sem symlink no caminho) com título, uma linha de status de gerado, toda seção que o padrão dele exige (tipo pelo `egm.doc_types`) com `TODO` e uma seção `## Sources` com links relativos ao documento. Diretórios no `--from` se expandem nos arquivos rastreados; arquivos não rastreados, negados, em symlink ou não regulares são pulados com o motivo; o esqueleto é recusado (`bootstrap_sources`) a menos que a coleta R7 inclua exatamente os arquivos aceitos. Nenhum conteúdo de arquivo vai para o rascunho (ele só é lido para essa checagem R7) e nada é colocado no stage nem commitado
- `tb egm drift-report` (Fase B1, `docs/specs/egm-drift-bootstrap.md`): compara cada documento sob gate com as fontes que a base dele certificou (o recibo commitado para esses bytes do documento, senão uma rodada local convergida) pelos hashes dos caminhos da base, com os status `fresh`, `no_baseline`, `sources_differ`, `drifted`, `source_missing` e `unreadable`, `--at` a partir dos blobs e `--json` (`egm-source-drift/0.1`). No `--egm-mode ci`, uma edição de recibo seleciona o documento dele, apagar o recibo de um documento que continua sob gate reprova (`receipt_removed`) e o `ci_source_drift` aponta o drift que o intervalo introduziu contra o recibo do merge-base (`egm.drift.ci: warn|fail`, chave de política). O aviso `sources_changed` do gate local agora usa os recibos também

### Corrigido
- `tb verify`: a allowlist de env removia `PLAYWRIGHT_BROWSERS_PATH`, então suítes E2E do Playwright falhavam só dentro do `tb` (estudo de campo: paprika). As variáveis de path de navegador (`PLAYWRIGHT_BROWSERS_PATH`, `PUPPETEER_CACHE_DIR`, `PUPPETEER_EXECUTABLE_PATH`, …) agora são liberadas pelo nome exato, e um resultado diferente de PASS lista `filtered_env` (só nomes, sem os que têm cara de segredo)
- `tb verify` / `guard` / `review` / `stack`: o `--base HEAD~1` implícito falhava num repositório com um único commit, com a dica `git fetch origin HEAD~1^{commit}` mesmo sem remoto. Quando o repositório só tem esse commit (sem clone raso nem branch órfã), agora cai para `HEAD` (diff vazio, aviso); refs relativas recebem uma dica de histórico curto em vez de `git fetch origin`
- `tb verify`: o `reason` mantinha escapes ANSI e os primeiros 512 bytes da saída, cortando o teste que falhou. Os filhos agora recebem `NO_COLOR=1 FORCE_COLOR=0`, o ANSI é removido e o trecho mantém as linhas de falha mais o resumo final; a mesma falha nomeada em duas iterações para com `ESCALATE` em vez de rodar de novo
- `tb graph`: imports de pacotes de workspace de monorepo (`pnpm-workspace.yaml`, `workspaces` do npm/yarn) viravam nós externos `pkg:`, então `callers`/`impact` não atravessavam pacotes; agora resolvem para os fontes do pacote via `exports` / `main` / `src/index`, inclusive re-exports de barrel. Imports relativos em arquivos `.tsx`/`.jsx`/Vue/Svelte/Astro/MDX/Angular não deixam mais um nó `pkg:./X`
- `tb review`: dentro do `tb stack`, critérios de toolchain ("build, testes e E2E passam") saíam sem evidência mesmo depois de o verify do mesmo stack dar PASS; esse veredicto agora conta. Findings de `quality-large-func` trazem `line`, e componentes React em `.tsx`/`.jsx` são medidos sem as linhas de markup JSX (limite de 120 linhas de lógica)
- `tb mem init`: não semeia mais o invariante de `.toolbelt/thresholds.yaml` quando o arquivo não existe (o guard continua protegendo o path)
- Varredura de escapes (Registry de `internal/sandbox`): um processo morto junto com um grupo escapado é reparentado ao subreaper ainda saindo, com o ambiente já ilegível; era reportado como escape sem atribuição, e o `tb egm` podia marcar uma rodada como não atribuída (`TestRunnerEscapeAttributed` intermitente). Um processo saindo (`PF_EXITING`) cujo ambiente já não pode ser lido agora é morto e colhido sem ser reportado, como um zumbi
- Varredura de escapes (Registry de `internal/sandbox`): um processo multithread escapado cuja thread líder chamou `exit(2)` enquanto as outras threads seguiam rodando aparece com estado `Z` e era tomado por zumbi: nunca classificado, nunca sinalizado, e o `wait4` não colhe um líder com threads vivas, então cada varredura girava até o prazo de 2 s com o mutex do Registry preso e o escape vivo nunca era morto nem reportado. Um líder `Z` com thread viva agora é classificado por essa thread (`/proc/<tid>`), morto e reportado; filhos zumbis também recebem `SIGKILL` (sem efeito num zumbi de verdade)

### Alterado
- `tb egm draft`: um rascunho novo e o `--update` agora usam os mesmos globs para o alvo (`egm.doc_globs` mais os globs extras do repositório; o `bootstrap` divide o helper), e `--body` com valor parecido com segredo é recusado (`draft_secret`) antes de escrever qualquer coisa. A R10 da spec agora exclui o diretório `drafts/` de propostas do `--update`
- Spec em rascunho da Fase B do `tb egm` (`docs/specs/egm-drift-bootstrap.md`): B1 `drift-report` determinístico (drift das fontes contra o recibo commitado, por hash dos caminhos) e checagens de CI contra o recibo do merge-base (`ci_source_drift`, edições de recibo selecionando o documento, `receipt_removed`); B2 `bootstrap` determinístico (um esqueleto com fontes em links, sem nunca sobrescrever); B3 os goldfish de drift e de bootstrap com LLM e um `commit` auxiliar no recibo

## [0.0.9] - 2026-11-20

### Adicionado
- B4b do barramento (`docs/bus-router.md`): slots do barramento `b1..bN` no `tb router run` a partir de `router.bus` (`channel`, `as`, `slots`, `to`, `claim_timeout`), lido só da config confiável; `max_workers: 0` para só barramento. Cada tentativa manda um assign `router-task/0.1` numa thread única `<ID>/<rand>.<attempt>`, só aceita claim de worker com o marker de confinamento feito depois do assign (do destinatário, quando fixado), revoga em retirada, lease perdido, timeout, kill, ticket alterado e desligamento (um resultado gravado antes vence) e, no PASS, confere HEAD/branch/base, faz o commit e exige a base como único pai. Novos `tb router run --check`, `Ticket.bus_thread`, `Result.bus_agent`/`bus_epoch`, campos do barramento no `state.json`, slots do barramento no `--dry-run`, `tb watchdog kill bN` e os alertas `bus_unclaimed`, `bus_claim_rejected`, `bus_lease_lost`, `bus_verdict_fail`, `bus_channel_expired`, `bus_write_failed`
- B4a do barramento (`docs/bus-router.md`): operações do store `Claim` e `Message` (leituras, também em canais expirados), `Revoke` (uma transação que devolve o estado final; uma thread concluída vence; threads revogadas recusam claim e complete e aparecem como perdidas no heartbeat) e `ClaimConfined` (marker de confinamento); um modo `router_tasks` do harness (um worker dedicado a tarefas: sem wakes comuns, com cursor próprio de tarefas) que só aceita assigns `router-task/0.1` de um agente fixado com papel router e caminhos validados, faz ele mesmo o claim e o complete fora do sandbox e roda o filho confinado ao worktree do ticket (`.git` e cópia do ticket só de leitura, sem acesso ao barramento, sessão nova do `claude`, só tools de arquivo) com uma vigia de revoke e um arquivo de veredito. O packet de wake ganha um objeto `task` opcional
- **Quebra:** o store do barramento migra para o schema v3 ao abrir (de mão única, como a v2: binários `tb` mais antigos passam a recusá-lo)

### Alterado
- `tb egm draft`: um rascunho novo e o `--update` agora usam os mesmos globs para o alvo (`egm.doc_globs` mais os globs extras do repositório; o `bootstrap` divide o helper), e `--body` com valor parecido com segredo é recusado (`draft_secret`) antes de escrever qualquer coisa. A R10 da spec agora exclui o diretório `drafts/` de propostas do `--update`
- O assunto do commit dos tickets do router tira caracteres de controle do título, para workers locais e do barramento
- Spec em rascunho do B4 do barramento (`docs/bus-router.md`), em duas partes. B4a: operações do barramento `Claim`, `Message`, `Revoke` (devolve o estado final) e `ClaimConfined` (schema v3), e um modo `router_tasks` do harness em que o harness faz claim e complete fora do sandbox enquanto o filho confinado só edita o worktree do ticket e escreve um arquivo de veredito. B4b: slots do barramento `b1..bN` no router, configurados só pela config confiável, com threads únicas por tentativa, validação do claim contra o assign e o marker de confinamento, e o router fazendo o commit do worktree depois de checar HEAD, branch e pai

### Adicionado
- Labs red+blue isolados `tb lab`, Fase 1 tier sandbox (ADR-017, spec `docs/specs/red-blue-labs.md`): `tb lab new|list|status|up|run|logs|down|verify` sobre um manifesto versionado `labs/<nome>/lab.yaml`, com resolução de tier (`auto|sandbox|podman`). O tier sandbox roda um cenário Go puro in-process (sem containers, sem sockets, sem env do host — garantido por teste de guarda de imports) que reproduz uma classe de vulnerabilidade e roda o check blue par, então `tb lab run sqli` reporta `attack=reproduced defense=detected`. O tier de topologia Podman falha fechado com remediação quando o Podman rootless está ausente (nunca degrada o isolamento); o estado de execução fica fora do repo; o aviso de uso autorizado é compartilhado com `tb red`. Novo `secplay.Corpus.ChecksForClass` exportado. Driver de topologia Podman, limites por cgroups, teste de egress por namespace, seeds via Containerfile, `tb lab export` e ferramentas MCP `tb_lab_*` são Fase 2
- Restante do B3 do barramento (`docs/bus-notify-discover.md`): o adapter de wake `notify` para agentes só de GUI (`desktop` via `notify-send` do root ou `osascript`, `terminal` no próprio processo; texto fixo em inglês só com contagens, tipos e nomes de remetente entre aspas; ambiente mínimo no filho; `urgent_urgency`; aviso `wake_mismatch`; campo `backend` na auditoria) e o `tb bus discover` (CLIs de agente conhecidas no `PATH` e configs de cliente MCP que rodam `tb mcp`, lidas com segurança; não executa nada). O packet de wake ganha as contagens `types` e `senders`

## [0.0.8] - 2026-11-20

### Adicionado
- Workflows de segurança no CI: CodeQL (`go` com build manual, `actions`, queries `security-extended`; em PR e semanal), OpenSSF Scorecard (semanal e quando a proteção de branch muda, resultados publicados, SARIF no code scanning) e zizmor (auditoria dos GitHub Actions em PRs que mudam workflows); nenhum roda após o merge. As actions deles ficam pinadas por SHA de commit
- B2 do barramento (lease, heartbeat, watchdog; spec `docs/bus-lease.md`): lease por agente (`tb bus channel create --lease`, padrão `2m`), fencing por epoch nos claims, tomada de claims liberados ou expirados, `tb bus heartbeat`, `tb bus watchdog` (só CLI, publica `lease_expired` do remetente exclusivo do broker `system:watchdog`), `channel show` com agentes e claims, tools MCP `tb_bus_heartbeat` e `tb_bus_status`, heartbeats em segundo plano do `tb mcp` para sessões ligadas e do harness `tb bus run` (auditoria `lease_lost`)
- Vault no `tb graph` (fatia 4 da Fase A do vault): com `TB_GRAPH=on`, notas viram nós `note:<caminho>` (títulos redigidos, proveniência de `source`/`confidence`), headings viram seções, wikilinks resolvidos viram arestas `links` e tags viram nós `tag:` com arestas `tagged`; os links Markdown e menções do corpo mantêm a nota como origem. O vault é lido só pelo `internal/vault`, nunca pelo indexador genérico, e as entradas dele (menos as com ponto) entram no fingerprint do índice; `vault.graph: false` desliga; `vault` entra no `graph.docs` padrão. Quebra consultas ao grafo: notas do vault eram nós `file:.toolbelt/vault/...` e agora são `note:<caminho>` (links para elas são remapeados); o indexador genérico deixa de percorrer o `.toolbelt/` do repositório
- `tb vault new` e `tb vault search`, e as tools MCP `tb_vault_list|show|search|links|backlinks|check|new` (fatia 3 da Fase A do vault): o `new` grava uma nota com o nome do slug do título redigido, frontmatter codificado em YAML, `O_EXCL`, sem symlink no `--dir`, e o `tb_vault_new` grava sempre `source: inferred` com `confidence` obrigatória; o `search` casa sem diferenciar maiúsculas e acentos sobre o texto redigido de cada nota
- `tb vault list|show|links|backlinks|check` e a op `vault` do `tb stack` (fatia 2 da Fase A do vault, só leitura): raiz e config lidas de `<raiz>/.toolbelt/config.yaml`; varredura segura (sem seguir symlinks, diretórios com ponto pulados, FIFOs nunca abertos, `O_NOFOLLOW|O_NONBLOCK` + `fstat`, `vault.max_note_bytes`); frontmatter tipado com número de linha; resolução de wikilinks do Obsidian com caminho parcial, anexos e `ambiguous_link` como erro; texto das notas dentro de objetos `data` e redigido; saída determinística. Chaves novas `vault.max_note_bytes`, `vault.types`, `vault.graph` com `VaultConfig.Validate`
- `internal/mdparse` (fatia 1 da Fase A do vault): parser Markdown folha com `Slugify`, `Headings` ATX cientes de cerca e de frontmatter, `SplitFrontmatter`, `Wikilinks` do Obsidian (rótulos, `\|` em tabela, fragmentos, embeds, fora de código inline) e `NormalizeForSearch`; ainda sem comando para o usuário

### Alterado
- `tb egm draft`: um rascunho novo e o `--update` agora usam os mesmos globs para o alvo (`egm.doc_globs` mais os globs extras do repositório; o `bootstrap` divide o helper), e `--body` com valor parecido com segredo é recusado (`draft_secret`) antes de escrever qualquer coisa. A R10 da spec agora exclui o diretório `drafts/` de propostas do `--update`
- **Quebra:** `tb bus complete` exige `--epoch` e `tb_bus_complete` exige `epoch` (o valor que o `claim` devolveu); o prompt do harness o passa. O store do barramento migra do schema v1 para o v2 ao abrir (de mão única: binários `tb` mais antigos passam a recusá-lo; atualize juntos todos os `tb` que servem um canal)
- Spec do B2 do barramento (`docs/bus-lease.md`): lease por agente com heartbeats, fencing por epoch nos claims (o `complete` recebe o epoch), watchdog sob demanda que manda `lease_expired` do remetente exclusivo do broker `system:watchdog`, heartbeats do harness e do `tb mcp`, e uma migração do store v1 → v2
- O vault passa a ser versionado: `.toolbelt/vault/` saiu de `LocalCacheExcludePatterns`; o `tb init` exclui só `<vault.path>/.obsidian/`, cria um `vault.path` customizado, avisa `vault_gitignored` quando uma regra do `.gitignore` ignora o vault e repara uma seção de exclude cujo marcador de fim se perdeu, sem descartar as linhas do usuário. O `.gitignore` deste repositório passa a ignorar só `.toolbelt/vault/.obsidian/`
- Seções Markdown do `tb graph`: linhas `#` dentro do frontmatter ou de bloco de código cercado deixam de ser seção, e uma sequência `#` de fechamento sai do texto do heading (regras ATX). Um arquivo Markdown que começa com uma quebra temática `---` passa a ser lido como frontmatter até a próxima linha `---` ou `...`, como no Obsidian

## [0.0.7] - 2026-11-20

### Adicionado
- EGM no CI (fatia 3 da Fase A.2): o workflow de exemplo `docs/examples/egm-ci.yml` (release do tb fixada e conferida com `sha256sum -c`, claude fixado, config do egm do CI escrita em `$RUNNER_TEMP`, SHAs de base/head do PR, `continue-on-error` até medir a instabilidade) e o guia `docs/egm-ci.md` (fluxo do autor, configurações do repositório com check obrigatório, "include administrators" e CODEOWNERS, tabela de falhas). O perfil `claude` autentica com `ANTHROPIC_API_KEY`/`CLAUDE_CODE_OAUTH_TOKEN` do `egm.env.pass` sem arquivo de credencial. No `--egm-mode ci` uma seleção vazia só avisa, a menos que `--strict` seja explícito
- `tb stack egm --egm-mode ci` (fatia 2 da Fase A.2, T-R2 a T-R8): confere o recibo commitado em head (`no_receipt`, `receipt_invalid`, `receipt_stale`), roda uma rodada nova `llm` num estado descartável com a config `-c` do próprio CI e casa os findings de `major` para cima com as alegações do recibo (`ci_finding`, `ci_downgrade`, alegações `arbiter_withdrawn` arbitradas de novo ou `ci_upheld`); `ci_offline` a menos que `--egm-allow-offline`, `ci_not_run`, `ci_budget` e `egm.ci.max_docs` (padrão 5, o repositório só pode baixar), `gated_doc_removed`; o passo informa `trust` e `ci_findings` por documento. O gate local passa a receber a config `-c` do egm e avisa `receipt_missing`/`receipt_stale`. O MCP `tb_stack` ganha `egm_mode`, `egm_config` e `egm_allow_offline`
- `tb egm receipt` e a ferramenta MCP `tb_egm_receipt` (fatia 1 da Fase A.2, T-R1): grava `.toolbelt/egm/receipts/<slug>.json` com os hashes do documento e das fontes e as alegações do autor (linhagens de `major` para cima que o árbitro retirou, recusadas ou encerradas à força, cada uma com `lines` no documento atual), com bytes determinísticos; recusado se o veredito não for `PASS` ou `PASS_PARTIAL`, quando o documento ou uma fonte difere de `HEAD` (`receipt_uncommitted`), e quando o caminho passa por um symlink (`receipt_path_untrusted`, gravado via `os.Root`)
- Gate do `tb egm` (Fase A, fatia 3): `tb egm status --at <rev>` avalia o documento, as fontes (modo do `ls-tree`, bytes do `cat-file blob`) e a config do repositório a partir do commit (R7a); a op `egm` do `tb stack` com `--egm-doc` e `--strict` seleciona os documentos sob gate adicionados ou modificados em `merge-base(base, head)..head` e os avalia no head, com `no_gated_docs`, `uncommitted_gated_docs` e `sources_changed`, e o campo aditivo `warn` no resultado do stack (R33); as tools MCP `tb_egm_status` e `tb_egm_resolve`, e `egm_docs`/`strict` no `tb_stack` (R31)
- Engine `llm` do `tb egm` (Fase A, fatia 2): os 9 revisores do preset consolidado como goldfish num sandbox Landlock (packet só de leitura, HOME e `TMPDIR` descartáveis, env mínimo, sem ferramentas, um Registry e um subreaper compartilhados, atribuição de órfãos), o perfil `claude` (flags sem ferramentas, cópia só do `claudeAiOauth` com checagem de `expiresAt`, detecção de uso de ferramenta) e o `custom`; personas e standards originais, o prompt da R9 com blocos de dados delimitados por nonce, validador escrito à mão com schemas JSON, a atestação da R17, retries, orçamento de execuções e `--dry-run`; matching de linhagens por similaridade (R19), a reavaliação pelo dono em que correção exige mudança (`fixed_elsewhere`, `fixed_without_edit`), e `--contest` com o árbitro (R28, R29). A build tag `egmfake` acrescenta o agente de teste `__fake-agent-egm`
- ADR-015 (proposed) e specs da Fase A do EGM nativo (`tb egm`): núcleo (`docs/specs/egm-nativo.md`, goldfish LLM isolados em sandbox com HOME descartável e env mínimo, sem ferramentas, defesas contra conteúdo de terceiros, convergência por conteúdo, árbitro fresco) e rascunho da spec de trust contra fraude ativa do mesmo usuário (`docs/specs/egm-trust.md`)
- `internal/sandbox`: campos `ReadOnlyWorkDir`, `PrivateTmp`, `Home`, `Path`, `Stdin`, `Stdout`/`MaxStdoutBytes` (stdout separado, excedente drenado) e `MinimalDev`; regras Landlock de arquivo sem direitos de diretório; comando absoluto sem `LookPath`
- `tb egm review|loop next|resolve|status` (Fase A, fatia 1, engine determinística): checkers `refs-exist`, `required-sections` e `plan-files-listed` e o scan anti-injeção, linhagens reavaliadas pelo re-run, `--fix`/`--decline`/`--force-settle`, convergência sobre o mesmo conteúdo, teto e `--restart`, vereditos da R25 e `report.{json,md}`. Git endurecido (lista fechada de chamadas, opções e env da R4c) e inspeção do `.git` com chaves fatais (`hostile_git_dir`, R4d); só config de usuário, o `.toolbelt/config.yaml` do repositório só pode endurecê-la (`config_untrusted`, R34). Estado em `$XDG_STATE_HOME/tb/egm/`. Engine `llm`, árbitro, `status --at`, a op do `tb stack` e as tools MCP vêm nas próximas fatias
- getters `mem.SecretPatterns()` e `graph.CollectMDLinks()` (sem mudança de comportamento)
- `bench/egm-spike`: spike R0 do `tb egm` (isolamento do goldfish com `claude -p` real), com os modos `collect`/`calibrate` e o corpus de matching `internal/egm/testdata/paraphrases.json`

### Alterado
- `tb egm draft`: um rascunho novo e o `--update` agora usam os mesmos globs para o alvo (`egm.doc_globs` mais os globs extras do repositório; o `bootstrap` divide o helper), e `--body` com valor parecido com segredo é recusado (`draft_secret`) antes de escrever qualquer coisa. A R10 da spec agora exclui o diretório `drafts/` de propostas do `--update`
- Spec de trust do EGM (Fase A.2, `docs/specs/egm-trust.md`) reescrita com o CI como fronteira de confiança: um recibo commitado com as alegações do autor, conferido num passo `tb stack --egm-mode ci` que roda uma rodada nova e independente com a config e a credencial do próprio CI; os nove mecanismos locais antifraude do primeiro rascunho saem
- ADR-015 aceito: status, autenticação, o teto `PASS_PARTIAL` offline, o limite `max_rounds` e os dados de convergência alinhados com os resultados da Fase A
- A saída da CLI e do servidor MCP passa a ser em inglês: help do cobra (`Short`/`Long`/`Example`), uso das flags, mensagens de erro, relatórios (ex.: `guard: ok (3 rules, 0 violations)`, drift `Out of scope` / `Forbidden touched`) e descrições das tools/schemas MCP; o parsing de entrada continua aceitando os títulos em português do `rd.md` (`Escopo`, `Proibido`, `Aceite`)

### Corrigido
- Build no Windows: o `tb egm receipt` usava `syscall.O_NOFOLLOW`, que não existe no Windows; a flag agora é só unix (o `os.Root` continua confinando a escrita)
- Op `egm` do `tb stack`: um índice `README.md` sob `egm.doc_globs` deixou de ser documento sob gate, então editar `docs/adr/README.md` não derruba o gate com `NO_STATE` (dogfood #38)
- `tb egm loop next` depois de uma convergência sobre o mesmo `content` e `policy_sha` não abre mais ciclo e rodada novos (cerca de 9 execuções de agente com a engine `llm`): mostra `already converged over this content in cycle N` com o relatório existente e sai com exit 0 (R26, dogfood #39)

## [0.0.6] - 2026-11-18

### Alterado
- `tb egm draft`: um rascunho novo e o `--update` agora usam os mesmos globs para o alvo (`egm.doc_globs` mais os globs extras do repositório; o `bootstrap` divide o helper), e `--body` com valor parecido com segredo é recusado (`draft_secret`) antes de escrever qualquer coisa. A R10 da spec agora exclui o diretório `drafts/` de propostas do `--update`
- Inglês passa a ser a língua padrão do repositório: docs da raiz (`README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `CHANGELOG.md`), `docs/`, `AGENTS.md`, `CLAUDE.md`, templates do `.github/`, invariantes e comentários de código; a tradução em português vai para `docs/pt-BR/` (substitui `docs/en/` e os `*.en.md` da raiz); `TestDocsTranslationMirrorComplete` substitui `TestDocsEnglishMirrorComplete`
- Landing page abre em inglês por padrão (toggle PT mantido)
- Comentários de código que só repetiam o que o código faz foram removidos; os restantes explicam como/por quê
- MCP: `tb_graph_search`/`callers`/`deps`/`impact`/`explain`/`path` nunca reindexam (agora `readOnlyHint`); índice obsoleto devolve `stale: true` e um `hint` para chamar `tb_graph_index`. A CLI continua respeitando `graph.index_on_change`
- MCP: o `initialize` negocia a versão do protocolo (`2025-11-25`, `2025-06-18`, `2024-11-05`): versão suportada volta igual, qualquer outra recebe `2025-11-25`; antes o servidor respondia sempre `2024-11-05`
- MCP: `tools/list` devolve as ferramentas ordenadas por nome (determinístico, cacheável)

### Documentação
- README, site, `docs/cli.md`, arquitetura, índice/status dos ADRs, protocolo do bus, release, CONTRIBUTING e SECURITY atualizados para a v0.0.5 (36 tools MCP, etapas B0–B4 do `tb bus`, `tb mcp --bus-as`, `graph explain`/`path`)

### Corrigido
- `tb init`: com config do golangci mas sem `golangci-lint` no PATH, `verify.lint_cmd` vira `go vet ./...` em vez de um comando que falharia (dogfood #1)
- `tb init` / `tb mem`: `config.yaml`, `agent_docs.yaml`, `invariants.yaml` e o `AGENTS.md` semeado são criados com `0644`, não `0600` (dogfood #7)
- `tb init` numa `git worktree`: o exclude do estado local vai para `$GIT_COMMON_DIR/info/exclude`, o único que o git lê (dogfood #17)
- `tb init`: dicas de `AGENTS.md`/`CLAUDE.md` ignoram comentários em fences de shell, comandos citados no meio de uma linha e nomes de ferramenta sem subcomando (`golangci-lint`, `go`) (dogfood #18)

### Adicionado
- MCP: toda ferramenta declara `annotations` (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) justificadas pelo efeito real do handler, conforme a revisão do M8ven (`docs/pt-BR/pesquisa/mcp-sinais-m8ven.md`)

### Segurança
- MCP: o `tb_run` roda sob Landlock: escreve só na raiz do repositório, no próprio `TMPDIR` e nos caches de toolchain; lê os diretórios do sistema, o `PATH`, o `GOROOT` e `sandbox.read`/`sandbox.write` da config global; `$HOME` e `/tmp` ficam fechados e `cwd` fora do repositório é recusado. Sem Landlock, falha fechado, a menos que `sandbox.allow_unsandboxed: true`. O resultado ganha `sandboxed`
- MCP: o `env.pass` (do `tb_run` e do gate de `pass_env`) e as chaves `sandbox.*` só são lidos da config global ou de `-c`, nunca do `.toolbelt/config.yaml` do repositório, que o agente pode escrever
- MCP: o argumento `pass_env` de `tb_run`, `tb_verify_run` e `tb_stack` só aceita chaves já presentes em `env.pass`; qualquer outra é recusada, então um modelo com prompt injection não consegue mais liberar segredos (`--pass-env` na CLI não mudou)

## [0.0.5] - 2026-11-18

### Adicionado
- Barramento B0/B1 (ADR-013): spec `bus/0.1` em `docs/bus-protocolo.md` (envelope JSON-RPC, Agent Card, convite) e `tb bus channel create|list|show`, `invite`, `join`, `send`, `recv`, `claim`, `complete`, `log` sobre `.toolbelt/bus/bus.db` (SQLite WAL via `modernc.org/sqlite`, sem CGO; diretório `0700`, banco e sessões `0600`)
- Barramento: convite de uso único com papel fixado (só SHA-256 no store), `from` definido pelo broker, dedupe por `idempotency_key`, TTL de mensagem e canal, cursor por agente com `--peek`/`--since`, long-poll até 25s, dono único por thread, payload ≤ 64 KiB com redação de segredos
- MCP: `tb_bus_join`, `tb_bus_send`, `tb_bus_recv`, `tb_bus_claim`, `tb_bus_complete` (identidade presa à conexão; criar canal/convite não é exposto)
- Barramento: `tb bus invite --name` vincula convite a um agente e recupera sessão perdida (token rotacionado); nome do dono e nomes com convite vinculado aberto ficam reservados
- Barramento B3: `tb bus run` (harness por agente; espera sem LLM via `PRAGMA data_version`, filtro `wake_on`, agrupamento com debounce/max_delay, orçamento de despertares, rewake/`wake_unread`, retry com backoff e `task.status` `wake_failed`) com adapters `exec` (comando local, wake packet no stdin, env filtrado + `pass_env`) e `harness` (preset `claude -p --resume --output-format stream-json`, sem `bypassPermissions`, `session_id` persistido); spec em `docs/bus-harness.md`
- `tb mcp --bus-as <canal>/<nome> [--bus-root] [--bus-state]`: liga a conexão MCP a uma sessão CLI do bus
- Barramento: sessões, arquivos de wake e estado do harness saem do repositório para `$XDG_STATE_HOME/toolbelt/bus/<hash>/` (`TB_BUS_STATE_DIR` sobrepõe); o agente acordado lê só o próprio token
- Sandbox: `Config.Stdin` (entrada padrão do filho, inclusive via trampolim Landlock)
- `tb bus run`: `wake_topics: false` ignora `topic:*`; o claude acordado usa `--strict-mcp-config` e temp próprio
- `tb init`: `.toolbelt/bus/` entra em `.git/info/exclude`
- `tb bus channel extend <canal> --ttl D`: redefine a expiração do canal (`0` = sem expiração) e reabre canal expirado mantendo sessões e histórico
- `tb graph explain <nó>` e `tb graph path <origem> <destino>` (MCP `tb_graph_explain`, `tb_graph_path`): vizinhança de um nó com proveniência e menor caminho entre dois nós, reimplementando as consultas do Graphify sem LLM; arestas `calls` &lt; 0.6 e alvos sem nó no índice ficam de fora
- ADR-014: Graphify — reimplementar, não integrar (fecha a questão aberta do ADR-006; docs de arquitetura deixam de citar tree-sitter como implementado)
- `tb graph search` (MCP `tb_graph_search`): ranking em Go puro — título exato, depois matches por substring (o que a busca antiga trazia), depois só termos; BM25 sobre título/path/pacote dentro de cada faixa, com quebra de camelCase/`snake_case` e consulta multi-termo

### Corrigido
- Sandbox: `isSubpath` tratava qualquer caminho relativo iniciado por `../` como interno, então `ValidatePath` aceitava caminhos fora do workdir (ex.: `/etc/passwd`) e a checagem de `ForbiddenPaths` não era confiável; agora só descendentes estritos contam
- Sandbox Landlock: removida a regra de escrita em `os.TempDir()` (`/tmp`), que por causa do bug acima nunca era aplicada; `/tmp` compartilhado segue sem acesso (nem leitura) quando não contém o workdir, e os docs deixam de prometer escrita lá
- Sandbox Landlock: libera leitura do diretório alvo do symlink de `/etc/resolv.conf` (ex.: `/run/systemd/resolve`); antes, filhos com rede não resolviam DNS
- `tb bus run`: `detail` do audit (stdout/stderr do filho) é redigido por valores conhecidos (`pass_env`, variáveis de segredo do harness, novo `secret_files`) e padrões genéricos (cabeçalhos de credencial, `segredo=valor`, credencial em URL, tokens longos); antes só `pass_env` e os padrões do `mem`
- `tb bus run`: com o canal expirado, suspende despertares e audita `channel_expired` uma vez (antes seguia calado); retoma ao `extend` (`channel_reopened`)

## [0.0.4] - 2026-11-17

### Adicionado
- Fase 3: `tb ticket` (`create`/`list`/`show`/`cancel`/`retry`; tickets YAML em `.toolbelt/tickets/`, `--from rd.md`, dependências com detecção de ciclo) e `tb board` (`list`/`watch`/`pause`/`resume`/`drain`; estratégias `fifo`/`priority`/`dependency`)
- Fase 3: `tb router run|status|config` — scheduler em primeiro plano com workers em worktrees próprias (`tb-workspace/<ticket>`), sandbox fail-closed (`--allow-unsandboxed` opt-in), entrega direta (subscriptions com TTL) e seção `router:` em `.toolbelt/config.yaml`; substitui o stub
- Fase 3: `tb watchdog status|alerts|kill` — alertas `worker_slow`, `worker_stuck`, `worker_crash`, `worker_killed`, `queue_growing` e `global_timeout` em `.toolbelt/router/alerts.jsonl`
- Fase 3: `tb merge` — integra tickets `done` de forma atômica; estratégias `fail` (padrão) e `last_wins`; `prompt`/`semantic` não implementadas
- MCP: `tb_ticket_list`, `tb_ticket_show`, `tb_ticket_create`, `tb_board`, `tb_router_status`, `tb_watchdog_alerts`, `tb_merge` (dry-run por padrão)
- Router: alertas `ticket_canceled` (ticket cancelado enquanto roda; agente morto, resultado descartado) e `multi_dep_base` (≥ 2 dependências sem merge ⇒ base em `HEAD`); ticket de dependência única parte do `head_commit` dela
- Router: resumo `stalled` + lista `unschedulable` (tickets `pending` bloqueados por dependências `failed`/`canceled`) e `tb router run` sai com código ≠ 0
- Router: um único `tb router run` por repositório (lock exclusivo `.toolbelt/router/lock`); verificação de integridade do worktree e git endurecido (fsmonitor/hooks off, `--no-verify`); saída do agente limitada a 1 MiB, logs 0600; exit 0 prevalece sobre veredito tardio de stuck/kill
- `tb merge` `last_wins`: commits de integração sem gpgsign/hooks, `--continue` com falha aborta sem alterar refs, sobreposição com `--no-renames`; MCP `tb_merge` devolve o relatório de conflito com `ok:false`
- `tb merge`: todo git endurecido (hooks/fsmonitor/diff externo off) e gate de caminhos sensíveis (hooks, `.toolbelt/**`, workflows, `.gitmodules`, gitlinks, `.gitattributes`, paths do guard) recusado sem `--allow-sensitive` (MCP `allow_sensitive`); JSON com `sensitive`
- Sandbox: `Config.Env` e `ProcessGroup` (mata o grupo de processos ao cancelar/expirar) para uso do router

### Corrigido
- Sandbox Landlock: `/dev/null`, `/dev/zero`, `/dev/full` passam a ser graváveis (e `/dev/random`/`/dev/urandom` legíveis) por regras por arquivo; antes `/dev` era só leitura e `echo hi > /dev/null`, `git` e shells falhavam (`cannot create /dev/null: Permission denied`; também na v0.0.3). `/dev` como um todo segue somente leitura; ttys (`/dev/tty`, `/dev/ptmx`, `/dev/pts`) ficam de fora de propósito. `IOCTL_DEV` só é tratado na ABI ≥ 5 (antes era aplicado a partir da ABI 4)

- Arquivos de estado (tickets/board, locks, `alerts.jsonl`) criados com modo 0600 (antes 0644); `tb merge` resolve o caminho absoluto do `git` uma vez em vez de buscar no `PATH` a cada chamada
- Testes de `internal/review` fixam a branch inicial (`git init -b master`); antes falhavam com `init.defaultBranch=main`

### Documentação
- `docs/cli.md`, README, arquitetura e ADR-009 atualizados para a Fase 3 (sem `router start/stop`, `router.yaml`, `watchdog scale`; merge via `tb merge TKT-001 TKT-002`)
- ADR-013 (proposta): barramento local multi-vendor (`tb bus`)
- Dogfooding: o repositório usa o próprio `tb` (`.toolbelt/`, invariantes, `.mcp.json`, `CLAUDE.md`) e registra atritos em `docs/campo/dogfood.md` (PT + EN)

## [0.0.3] - 2026-11-17

### Adicionado
- `tb verify run` / `tb stack`: flags `--build-cmd`, `--lint-cmd`, `--no-build`, `--no-lint` e `--dry-run` (plano com `dir`/`commands` sem executar); MCP `tb_verify_run` / `tb_stack` com os mesmos campos
- JSON de verify e de cada etapa do stack expõem `reason`/`error` no ESCALATE/FAIL (não só DEBUG no stderr)
- `tb stack --jobs N`: concorrência para etapas somente leitura (`guard`/`drift`/`review`);
  default = nº de CPUs (máx. 32); `--jobs 1` força serial; MCP `tb_stack` aceita `jobs`
- Release: SBOM (syft via `sboms:` no GoReleaser), assinatura keyless cosign do `checksums.txt` (OIDC GitHub) e atestação de proveniência SLSA (`actions/attest-build-provenance`)
- CI: job `govulncheck` em todo push/PR
- CI: Dependabot semanal (segunda) para módulos Go e GitHub Actions, com updates agrupados
- Espelho completo da documentação em inglês (`README.en.md`, `*.en.md` na raiz,
  `docs/en/**`) com seletores de idioma; PT permanece a fonte de verdade
  (`docs/en/TRANSLATION.md`, `docs/TRANSLATION.md`)
- Teste `TestDocsEnglishMirrorComplete` exige par em `docs/en/` para cada `.md` em
  `docs/`; `TestDocsCLIExamplesMatchCobra` também valida `docs/en/cli.md`
- `tb sandbox run --allow-unsandboxed`: opt-in explícito para executar sem isolamento Landlock
- Teste de integração que tenta escrever fora do workdir (pula se o kernel não tiver Landlock)
- Grafo TS/JS: fingerprint no `graph.json` e detecção de índice desatualizado;
  com `graph.index_on_change: true` (default) `search`/`callers`/`deps`/`impact`/`stats`
  reindexam automaticamente; com `false`, avisam na saída e expõem `stale: true` no JSON
- Resolução de `tsconfig.json`/`jsconfig.json` (`extends`, `baseUrl`, `paths`) no indexador JS/TS
- Landing page estática em `site/` (HTML/CSS/JS, tema Synthwave, PT/EN) publicada via
  GitHub Pages (`https://leonardo-moraes360.github.io/toolbelt/`)
- Workflow `.github/workflows/pages.yml` (configure/upload/deploy-pages; sem `enablement`)
- Guia `docs/site.md` (preview local e como habilitar Pages em Settings)
- Flag global `tb --version` (mesma saída textual de `tb version`)
- `tb init` grava padrões de cache/derivados em `.git/info/exclude` (grafo, logs,
  vault, workspaces, captures, bench) sem tocar no `.gitignore` do projeto;
  invariantes/config permanecem versionáveis (recomendação documentada em
  `docs/cli.md`)
- `tb mem init` reporta caminho e invariantes criadas (ou já existentes); JSON com
  `created`/`count`/`invariants`

### Mudanças
- Dependências: `cobra` v1.10.2, `pflag` v1.0.9, `golang.org/x/text` v0.44.0, `golang.org/x/sync` v0.25.0; Actions do CI/Release/Pages atualizadas
- `tb stack`: etapas `guard`/`drift`/`review` rodam em paralelo (errgroup); `verify` permanece
  isolado; JSON preserva a ordem pedida; `--fail-fast` cancela as demais via context;
  `Prepare` paraleliza git/IO independentes
- Instalação por binário no README exige `cosign verify-blob` + `sha256sum -c` (remove `curl | tar` e a menção ao Homebrew sem tap)
- Go atualizado para `1.27.4` (`go.mod`, workflows CI/Release); golangci-lint no CI passa a v2.15.0 (compatível com Go 1.27)
- `docs/release.md` documenta SBOM, assinatura, atestação e validação local com `--skip=sign`
- `tb review` / `tb drift` / `tb stack`: `--approved-sha` passa a usar
  `git patch-id --verbatim` (sensível a whitespace); JSON sempre expõe
  `pinned_approval.checked` (`false` quando a checagem não rodou);
  `--require-approved-sha` e config `review.require_approved_sha` falham se a
  flag estiver ausente; base/head são resolvidos para SHA uma vez no início
  (anti-TOCTOU) e reutilizados no fluxo
- **Breaking (JSON grafo):** consultas MCP/CLI (`search`/`callers`/`deps`/`impact`) passam a
  emitir `{"stale": bool, "nodes": [...]}` em vez de um array nu; `stats`/`index` incluem `stale`
- Indexador JS/TS indexa `*.test.*`, `*.spec.*`, `__tests__` e `e2e` (antes eram omitidos);
  `impact`/`callers` passam a listar testes que importam o símbolo
- Saídas JSON usam listas vazias `[]` (nunca `null`) para `violations`, `out_of_scope`,
  `forbidden_hit`, `missing` e resultados vazios de `tb mem list|search`
- `tb init --json` e `tb mem init --json` emitem payload do que foi criado/detectado
  (path, cmds detectados / invariantes)
- `hidden_passed` é `null` quando não há suíte oculta (`hidden_available: false`)
- `duration_ms` / `total_duration_ms` no `tb stack` passam a float com fração sub-ms
- Erros de preparação/operação do `tb stack` ficam em `steps[].error` dentro do JSON
  (incluindo op desconhecida); não só no stderr
- MCP: `all` em `tb_mem_list` / `tb_mem_search` / `tb_mem_check` tipado como boolean
  (default `false`); handlers ainda aceitam strings legadas `"true"`/`"false"`
- `tb stack` sem operação: erro curto listando ops válidas (`verify`, `review`,
  `drift`, `guard`), sem dump de `Usage` (alinhado ao padrão de erros de domínio)
- Help/`docs/cli.md` de `tb stack` deixam explícito que empilha operações de
  verificação e **não** é stack de branches (Graphite/git-stack)
- `tb mem add|edit|list`: rejeitam `--category` inválida e `--ref` fora de
  ADR/decisions, com lista dos valores válidos no erro
- `tb mem search`: busca case- e accent-insensitive (Unicode NFD)
- Docs: `tb drift` deixa de ser descrito como “vault vs código”; o relatório real é
  escopo / forbidden / commit base (+ `pinned_approval` opcional)
- Docs: deixa explícito que o `tb` **não fala com o GitHub** (100% local) e precisa
  de um `rd.md`/ticket no disco; exemplos de como gerar a partir de issue/PR via `gh`
- Docs: linguagem `graph.languages: javascript` cobre TypeScript (`ts`/`tsx`); não há
  linguagem `typescript` separada
- Docs: anota a depreciação de `tb guard check --strict` (substituído pelo default
  estrito e por `--warn-only`); remove o exemplo invocável com `--strict`

### Corrigido
- Stack/verify respeitam overrides de comando: `--test-cmd` + `--no-build`/`--no-lint` (ou `--build-cmd`/`--lint-cmd` vazios) pulam o `build_cmd`/`lint_cmd` do config
- `tests/hidden/` sem toolchain aplicável deixa de forçar `go test` (marca `hidden_available: false`); detecta Go/Python/JS quando há suíte
- Build/lint bem-sucedidos são cacheados na mesma execução; falha determinística de build/lint não entra no retry de `--max-iter`
- `tb review` / `tb drift` / `tb stack` (e MCP `tb_review` / `tb_drift` / `tb_stack`):
  ticket inexistente ou ilegível (`--ticket` / `ticket`) passa a falhar com exit ≠ 0
  e mensagem clara citando o caminho procurado (`ticket não encontrado: …` /
  `ticket ilegível (…): …`), em vez de seguir com exit 0
- Refs de `--base`/`--head`/`--approved-sha` (CLI e MCP em guard/drift/review/verify/stack)
  rejeitam valores que começam com `-`, resolvem via
  `git rev-parse --verify --end-of-options <ref>^{commit}` e usam o SHA daí em
  diante; chamadas `git` passam `--end-of-options` onde aplicável. SHA de 40 hex
  inexistente agora falha com erro claro (`ref inexistente ou inválida`)
- `tb review` / `tb drift` / `tb stack`: `--approved-sha` compara
  `approved_base..approved` vs `head_base..head` (não a mesma `--base` nos dois
  lados); default das bases via `git merge-base`; nova flag `--approved-base`;
  range-diff só com `=` (e patch-id igual) conta como match após rebase limpo;
  JSON de `pinned_approval` expõe `approved_base` e `head_base`
- **Sandbox Landlock (fail-closed):** probe de ABI usa `LANDLOCK_CREATE_RULESET_VERSION` (antes `flags=0` → `EFAULT` e falso “sem Landlock”); `PR_SET_NO_NEW_PRIVS` antes de `landlock_restrict_self`; rights FS completos por ABI (incl. truncate/make_*); ruleset aplicado no filho via re-exec antes do `exec` do comando; erros de ruleset não são mais engolidos; sem Landlock a execução recusa com erro claro (use `--allow-unsandboxed`)
- `tb review` (goldfish): detecta só `TODO`/`FIXME`/`XXX`/`HACK` como palavra
  inteira em maiúsculas em comentários (ignora “todo” em português)
- `tb review` acceptance: cada achado liga `criterion` + `evidence` do diff;
  considera testes adicionados (`Test*` em Go; `test`/`it`/`describe` em JS/TS)
- `tb review` invariants: aplica regras de mandate (ex. proibir `console.log`,
  sem comentários) às linhas adicionadas do diff
- `tb review`: saída texto usa `papel=<role>` (e `papéis_acordo` se >1) no lugar
  de `acordo=N/M`; JSON inclui `role` (e `criterion`/`evidence` no aceite)
- Funções genéricas TS (`function foo<T>(…)`) voltam a aparecer no índice
- Imports relativos estilo NodeNext (`.js`→`.ts`/`.tsx`, sem extensão, `index`) deixam de
  ficar como `pkg:./…` e resolvem para `file:…`
- Aliases de import (`import { a as b }`) e namespaces (`import * as ns`, `ns.fn()`) alimentam
  arestas `calls` resolvíveis
- Erros de git em `tb guard` / `tb review` / `tb drift` / `tb verify` deixam de
  aparecer só como `exit status 128/129`: incluem stderr do git, a ref ausente e
  dica `git fetch origin <ref>` (helper central `internal/git`)
- `--approved-sha` igual à base não despeja mais o usage do `git range-diff`;
  mensagem clara sobre intervalo `base..approved` vazio
- `tb verify run` com comando inexistente (`test_cmd` / `build_cmd` / `lint_cmd` /
  `hidden_test_cmd`) sai com código **127**; com `--json` emite o envelope
  `verify.Result` (inclui `error`) em vez de sair sem JSON
- `tb drift` / `tb review` / `tb stack`: diff de arquivos passa a usar three-dot
  (`base...head` / merge-base), evitando falsos `out_of_scope` após merge da main
  ou squash de outro PR na base
- `tb drift`: `--base` explícito na CLI/MCP não é mais sobrescrito por
  `base_commit` do ticket; `base_mismatch` usa `git merge-base --is-ancestor`
  no `base_commit` declarado (ou na base efetiva se ausente), em vez de
  igualdade de string com `--base`; JSON inclui `actual_base` e `merge_base`
- `tb verify run --base/--head`: quando o head resolve para um SHA diferente do
  `HEAD` do worktree, build/lint/test/hidden rodam num `git worktree add --detach`
  temporário (removido ao final, inclusive em erro/sinal); o JSON expõe `head_sha`
  e `workdir`. Se `head == HEAD` com worktree sujo, emite aviso. O mesmo vale para
  `tb stack verify` via contexto compartilhado.

### Segurança
- Processos filhos de `tb verify` / `tb stack` (verify) e MCP `tb_run` /
  `tb_verify_run` / `tb_stack` passam a receber ambiente filtrado por allowlist
  (PATH, HOME, LANG, TMPDIR, GOPATH/GOCACHE, NODE/npm/pnpm, etc.). Segredos
  (`*TOKEN*`/`*SECRET*`/`*KEY*`/`*PASSWORD*`) nunca passam sem opt-in explícito
  (`--pass-env` / `env.pass` / arg MCP `pass_env`). Extensão via `--env` /
  `env.allow` / arg MCP `env`.
- `tb mem`: invariantes `immutable` ou categoria `anti_cheat` exigem `--force` + `--reason` em `edit`/`retire`; histórico append-only em `.toolbelt/invariants_history.jsonl`
- `tb mem add`: novas invariantes nascem **mutáveis**; use `--immutable` (MCP: `immutable`) para proteger; `mem init` continua criando defaults imutáveis
- `tb mem check`: falha **fechado** (arquivo ilegível/corrompido, SHA divergente, IDs ausentes, regra não avaliável → exit ≠ 0); não auto-inicializa defaults
- Redação de segredos (`ghp_`/`gho_`/`github_pat_`, `AKIA`, PEM, `Bearer`, `xox*`, `*_TOKEN=`) em `mem show`/`list`/`search` (CLI `--json` e MCP)

## [0.0.2] - 2026-11-16

### Adicionado
- `tb stack <op> [<op>…]` empilha `verify`/`review`/`drift`/`guard` numa execução com
  contexto compartilhado (diff, índice de arquivos, memória e config); JSON agregado
  (`ok`/`steps`/`total_duration_ms`); `--fail-fast`; MCP `tb_stack` com schema tipado
- MCP `tb_graph_impact` (paridade com `tb graph impact`)
- `tb review` / `tb drift`: flag `--approved-sha` compara `git patch-id --stable`
  de `base..approved` vs `base..head`; em divergência mostra `git range-diff` e
  sai ≠ 0; JSON registra `pinned_approval` (`approved_sha`, `patch_id`,
  `head_patch_id`, `match`, `range_diff`)
- Indexador JS/TS emite arestas `calls` resolvíveis (chamadas + JSX `<Comp />`) via imports relativos; `tb graph callers`/`impact` funcionam em fixtures Next/React (`pages/` → `components/`/`lib/`)
- `tb mem show|edit|retire|search` e vínculo `--ref` com ADR/decisions.md; soft-retire omitido de list/check/search (use `--all`); subcomando desconhecido sai ≠ 0
- MCP: `tb_mem_show`, `tb_mem_edit`, `tb_mem_retire`, `tb_mem_search` (+ `ref`/`all` em add/list/check)

### Mudanças
- Schemas MCP tipam `depth`/`limit`/`reviewers`/`max_iter` como integer, `strict`/`warn_only`/`transitive`/`skip_unavailable` como boolean, `threshold` como number e `standards` como array; handlers ainda aceitam strings legadas
- **Breaking:** saídas JSON da CLI (`--json`) e dos payloads de domínio no MCP usam
  exclusivamente chaves `snake_case`. Structs sem tag (`sandbox.Result`,
  `mem.Invariant`, `workspace.Workspace`/`Scope`) deixam de emitir PascalCase
  (`ExitCode` → `exit_code`, `TimedOut` → `timed_out`, `duration_ns`, etc.).
  Envelopes do protocolo MCP (`isError`, `inputSchema`, …) permanecem camelCase.
- Frameworks (ex. `react` em `.tsx`) rodam em paralelo com o indexador de linguagem, não no lugar dele
- `verify.build_cmd` em Node também aceita scripts `typecheck` / `type-check` quando
  `build` está ausente
- `tb guard check` sai com exit ≠ 0 por default quando há `SeverityError` (CI-safe); opt-out via `--warn-only`; `--strict` permanece como alias deprecated
- Modo texto limpo imprime `guard: ok (N regras, 0 violações)`; JSON/MCP incluem `rules_checked`
- MCP `tb_guard_check`: `strict_fail` por default em SeverityError; `warn_only=true` para opt-out (`strict` deprecated)
- `tb review`: goldfish distintos por papel (`scope`, `quality`, `acceptance`) em vez do mesmo checker clonado N vezes; `--reviewers` só limita papéis; qualidade cobre TODO/FIXME, funções grandes e código sem testes; aceite exige evidência em testes/arquivos de código

### Corrigido
- `tb guard check`: renames (`R*`) checam origem e destino; matching de
  `protected_paths` é case-insensitive; globs com `**` funcionam; regras
  inválidas não entram em `rules_checked`; config lida da **base**
  (`git show base:.toolbelt/guard.yaml`) e alteração nesse arquivo é
  violação; YAML malformado falha fechado (exit ≠ 0); varredura usa o
  diff/`git show head:path` (não o working tree); detecção de enfraquecimento
  de testes JS/TS (`skip`/`only`/`xit`/asserções triviais)
- Descrição longa do CLI: completa o adjetivo em pt-BR (`persistent` → `persistente`)
- `docs/cli.md` alinhado à CLI real: remove `graph callers --file`; `router` documentado como stub; seções planejadas (`proc`/`agent`/`watchdog`/`merge`) marcadas; `init`/`sandbox`/`workspace`, mem show/edit/retire/search, guard `--warn-only`, review standards e códigos de saída conferidos contra `--help`/MCP
- Teste `TestDocsCLIExamplesMatchCobra` extrai `tb …` de `docs/cli.md` e valida subcomandos/flags na árvore Cobra
- `tb verify run` falha imediatamente (sem retries) quando o comando configurado não existe (exit 127), não é executável (exit 126) ou `exec` retorna `ErrNotFound`, com mensagem `comando não encontrado: … — ajuste verify.test_cmd` (e equivalentes para build/lint/hidden)
- `tb init` em repos Node/TS sem `go.mod` não mantém mais `verify.test_cmd: go test ./...`:
  detecta o package manager (`pnpm-lock.yaml`/`yarn.lock`/`package-lock.json`/`bun.lockb`),
  preenche `test`/`build`|`typecheck`/`lint` a partir de `package.json`, e se nada for
  detectável deixa `test_cmd` vazio com aviso; o seed de `AGENTS.md` reflete isso
- CLI: erros esperados de `RunE` (domínio/runtime) não imprimem mais o dump de `Usage` do Cobra no stderr; `Usage` permanece só em erros genuínos de parse de flags/args. Subcomandos desconhecidos em qualquer nível saem com exit ≠ 0 e mensagem clara.
- Erro de grafo desabilitado (CLI e MCP) cita como ligar: `--enable` ou `TB_GRAPH=on`

## [0.0.1] - 2026-11-16

### Adicionado
- CI de release: GoReleaser (`.goreleaser.yaml`) + workflow em tags `v*` publicando binários `tb` (linux/darwin/windows × amd64/arm64), `checksums.txt` e changelog
- Documentação de instalação via GitHub Releases e guia `docs/release.md`
- Versão padrão `dev` injetável via ldflags (`tb version` / MCP `tb_version`)
- `tb init --write-agents` semeia `AGENTS.md` citando a toolchain detectada (`verify.test_cmd` e, se presentes, `build_cmd`/`lint_cmd`)
- MCP: tools de paridade CLI `tb_mem_*`, `tb_verify_run`, `tb_review`, `tb_drift`, `tb_guard_check`
- Indexador Rust emite arestas `calls` mínimas resolvíveis no repositório (`tb graph callers` / `impact`)
- `tb graph callers`/`impact` retornam erro explícito em C/C++ (sem arestas `calls`) em vez de `(vazio)` silencioso
- `tb init` preenche `verify.build_cmd` e `verify.lint_cmd` a partir da toolchain (scripts `build`/`lint`, golangci, ruff/mypy, `cargo build`/`clippy`, targets make)
- `tb guard check` aceita `--base`/`--head` (como review/drift/verify) além do intervalo posicional `base..head`
- Indexador Python emite arestas `calls` resolvíveis no repositório (`tb graph callers` / `impact`)
- `tb init` lê `AGENTS.md`/`CLAUDE.md` na raiz (registra em `.toolbelt/agent_docs.yaml` e extrai dicas de comandos; nunca sobrescreve) e aceita `--write-agents` para semear um `AGENTS.md` mínimo apontando ao tb
- `tb guard` carrega `protected_paths` de `.toolbelt/guard.yaml` (unidos aos defaults; arquivo ausente mantém só os defaults)
- `tb init` detecta toolchain (`go.mod`, `package.json`/npm/pnpm/yarn, pyproject/uv/pytest, `Cargo.toml`, `Makefile`) e preenche `verify.test_cmd`
- Ruler visual em `tb verify run --ruler visual` (screenshot headless, similaridade pixel, skip gracioso)
- `tb review` fresh-context (Elephant-Goldfish) contra ticket/rd.md e invariantes de `tb mem`
- `tb drift` para mudanças fora do scope declarado e divergência de commit base
- `tb graph` sob demanda (ADR-011): index/search/callers/deps/impact/stats atrás de `TB_GRAPH`/`--enable`
- Ferramentas MCP de grafo: `tb_graph_search`, `tb_graph_callers`, `tb_graph_deps`, `tb_graph_index`
- Comparação A/B em `tb bench compare` (configs YAML / feature flags no mesmo task set)
- Métricas de sucesso, tempo e tokens no JSONL e no relatório Markdown A/B
- Fixtures realistas: `localize-callers`, `cross-file-rename`, `api-handler`
- Configs A/B de exemplo em `bench/ab/` (`graph-on`, `graph-off`, `baseline`)
- CLI skeleton com Cobra (`tb version`, `tb init`, `tb help`)
- Configuração hierárquica (global `~/.config/toolbelt` + local `.toolbelt/`)
- Logging estruturado com `log/slog` e tema Synthwave
- Suporte a `NO_COLOR` e `TERM=dumb` para ambientes sem cor
- Flags `--json` para saída estruturada
- `tb bench run` - harness para executar benchmarks contra agentes
- `tb bench list` - lista tarefas de benchmark disponíveis
- `tb bench report` - gera relatório Markdown de resultados
- `tb bench compare` - compara dois benchmarks
- Agente fake (`fake-agent`) para testes no CI sem LLM
- 5 tarefas fixture: `bugfix-go`, `feature-small`, `refactor-simple`, `test-coverage`, `doc-function`
- Resultados em formato JSONL para análise programática
- Métricas: pass@1, pass@k, tokens, tempo, tool calls
- Makefile com targets: `build`, `test`, `lint`, `bench-fake`
- GitHub Actions CI: test, lint, build, bench-fake
- Configuração golangci-lint
- Configuração goreleaser para releases
- ADR-012: Framework CLI - Cobra
- Stubs para comandos futuros: `sandbox`, `verify`, `mem`, `graph`, `router`, `mcp`

### Corrigido
- `tb graph callers`/`impact` em Python: resolve por sufixo de nome qualificado e ignora arestas `calls` com confiança &lt; 0.6 (reduz alvos espúrios como `setup.py`→`tests.setup`)
- docs/cli.md alinhado à CLI real: `graph index [path]` (sem `--path`), sem `graph query`, `review`/`drift` só `--output` (sem `-o`), flags de graph/mem verificadas contra `--help`
- `tb drift` resolve refs git (`HEAD^`, tags, SHA curto) via `rev-parse` antes de
  comparar com `base_commit` (evita falso `base_mismatch`)
- `tb verify run --json` sai com exit ≠ 0 em `ESCALATE`/`FAIL` (alinhado ao modo texto)
- `tb init`: hints de `AGENTS.md`/`CLAUDE.md` ignoram fences de código longos e
  aceitam prosa curta (“use/run/prefer …”); `verify.test_cmd` valida o binário no
  `PATH` com fallback (ex. `uv` ausente → `pytest` / `python -m pytest`)
- `tb bench`: honra `bench.tasks_dir` / `fixtures_dir` / `output_dir` do config.yaml
  (precedência: flag CLI > YAML > default); a flag global `-c/--config` alimenta `config.Load()`
- `protected_paths` com barra final (`secrets/`) passa a proteger filhos do diretório
- `tb review`: invariante `tests/hidden` (e `thresholds.yaml`) casa por prefixo/segmento
  de path nos arquivos alterados, não por substring no diff (evita FP quando o ticket
  só menciona a regra)

### Mudanças
- README atualizado com instalação, uso rápido e comandos implementados
- docs/cli.md marcando comandos implementados
- docs/benchmark.md com seção sobre execução na Fase 0

## [0.0.0] - 2026-11-15

### Adicionado
- Documentação de arquitetura completa
- ADRs 001-011 definindo decisões técnicas
- Roadmap em 3 fases (MVP, Hierarquia de Agentes, Ferramentas Avançadas)
- Pesquisa de fontes primárias (100+ papers e repositórios)
- Identidade visual Synthwave
- Templates de issues e PRs
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md
