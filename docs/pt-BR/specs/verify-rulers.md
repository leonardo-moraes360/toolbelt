[🇺🇸 English](../../specs/verify-rulers.md) | 🇧🇷 Português

# Spec: réguas adicionais para o `tb verify`

Autor: toolbelt · Status: Implementada (rodada 2) (fatia 1: R0–R3, R5, R6; fatia 2: R4) · Data: 2026-11-22 · Registro de decisão: [ADR-008](../adr/008-verificacao-adversarial.md) (réguas plugáveis) · Relacionados: [ADR-005](../adr/005-anti-trapaca.md) (anti-trapaça), [verificacao-visual.md](../verificacao-visual.md) (a régua visual)

## Visão geral

O `tb verify run` julga uma mudança com uma de duas réguas: `tests` (build, lint, testes visíveis, testes ocultos) ou `visual`. O estudo de campo achou a lacuna que importa: **o verify diz PASS enquanto o gate do próprio projeto falha**. Esta spec acrescenta três coisas:

- **passo `typecheck`** na régua `tests`: um verificador de tipos é um gate próprio, não um build nem um lint;
- **checks nomeados** na régua `tests`: gates extras que o usuário declara (um scan de vulnerabilidades, `clippy`, uma checagem de licenças);
- **régua `coverage`**: cobertura de linhas igual ou acima de um limiar que a mudança não consegue baixar.

Ela também lê os comandos do verify a partir da ref base, para que a mudança em teste não possa reescrever o próprio gate. Torna as réguas combináveis (`--ruler tests,coverage`), corrige um `--ruler` desconhecido que roda `tests` em silêncio e liga tudo isso ao `tb stack` e ao MCP.

## Estado atual (2026-11-22)

| Item | Estado |
|------|-------|
| Seleção de régua | `cmd/tb/verify.go:50` e `internal/registry/parity.go:490`: `visual` vai para `runVisualVerify`, **qualquer outra coisa roda `tests`** (uma régua desconhecida não é erro) |
| Abstração de régua | Nenhuma; cada régua devolve o próprio payload; `emitVerifyResult` é o único ponto em comum |
| Passos de `tests` | build → lint (cada um uma vez, em cache; uma falha é determinística: `ESCALATE`) → testes visíveis → testes ocultos, por iteração |
| Confiança nos comandos do verify | `test_cmd`, `lint_cmd`, … vêm do `.toolbelt/config.yaml` da árvore de trabalho (`config.Load()`), que o guard não protege: a mudança em teste pode reescrever `test_cmd: true` |
| Veredictos | `PASS`, `FAIL`, `ESCALATE`, `SKIPPED`; só `PASS` e `SKIPPED` saem com 0 |
| `Result.metrics` | `map[string]string`, nunca preenchido |
| `VerifyConfig` | `test_cmd`, `hidden_test_cmd`, `build_cmd`, `lint_cmd`; a mesclagem com a config de arquivo está copiada em três lugares (`cmd/tb/verify.go`, `parity.go`, `stack.go`) |
| Códigos de saída | `main.go` sai com 1 em qualquer erro; o 127 de comando ausente vem do resultado, não de um erro |
| Script `typecheck` do Node | Detectado pelo `tb init` como `build_cmd` (`internal/config/detect.go:134`), então é reportado como build |
| `.toolbelt/thresholds.yaml` | Caminho protegido pelo guard (`internal/guard/guard.go:73`) e objeto de uma invariante anti-trapaça; **nada o lê** |
| Operação verify do `tb stack` | Sempre a régua `tests`; sem parâmetro de régua |
| Execução | `sh -c` com a allowlist do execenv, sem sandbox (inalterado aqui) |

## Demanda (estudo de campo)

| Necessidade | Onde |
|------|-------|
| Typecheck como gate: "vitest PASS + verify PASS; `tsc` falha" | [excalidraw.md](../campo/excalidraw.md) (P0), [pydantic-ai.md](../campo/pydantic-ai.md) (régua composta, P2) |
| Verify PASS sem o lint/build/typecheck do projeto | [SINTESE.md](../campo/SINTESE.md), [rodada2/SINTESE.md](../campo/rodada2/SINTESE.md) C9, click, mcp-python-sdk, ky, ollama, crush |
| Scans de segurança que o gate local não viu (`govulncheck`, `clippy`) | [paprika.md](../campo/paprika.md) |
| Cobertura | fraca (requests, express, scripts do mcp-python-sdk); por isso fica pequena |
| Teste de mutação | nenhum repositório usava ([SINTESE.md](../campo/SINTESE.md)); fora do escopo |

## Objetivos

- G1: uma falha do verificador de tipos reprova o verify, reportada como `typecheck`, não como build nem lint.
- G2: o usuário pode declarar gates extras por nome, e cada um é reportado separadamente.
- G3: uma régua de cobertura cujo limiar vem da ref **base**, para que a mudança em teste não consiga baixá-lo.
- G4: as réguas se combinam: `--ruler tests,coverage` roda as duas e o veredicto é o pior entre elas.
- G5: nenhuma mudança de comportamento sem config nova: `tb verify run` com a config e as flags de hoje dá o mesmo veredicto e as mesmas chaves JSON. Toda chave nova é `omitempty` e só aparece quando o passo dela rodou. A única mudança intencional: uma mudança que edita `verify.*` no `.toolbelt/config.yaml` é detectada (R0).
- G6: o gate não pode ser reescrito pela mudança que ele julga: os comandos e limiares do verify vêm da ref base.

## Não-objetivos

- Teste de mutação, detecção de testes instáveis, regressão de desempenho, checagens de compatibilidade de API.
- Detectar testes sem asserções (`artificial_coverage` do ADR-005): aqui a cobertura é um piso, não prova de teste.
- Colocar os comandos do verify em sandbox (rodam como hoje).
- Fixar o que os comandos fixados na base leem: eles ainda podem rodar arquivos que a mudança edita (alvos do Makefile, scripts do `package.json`, arquivos de teste). O R0 fixa quais comandos rodam, não o que eles leem; testes ocultos e os caminhos protegidos do guard são a defesa para isso (G6).
- Rodar a suíte da ref base para comparar cobertura (um modo "sem queda"); o limiar é um piso fixo.

## Glossário

- **Passo:** um comando dentro da régua `tests` (build, typecheck, lint, check, visíveis, ocultos).
- **Passo de gate:** um passo que roda uma vez por execução do verify e fica em cache entre iterações (build e lint hoje; typecheck e checks aqui). A falha dele é determinística para o loop do agente: o verify termina com `ESCALATE`.
- **Limiares base:** o `.toolbelt/thresholds.yaml` como está commitado na ref base.

## Design

### R0. A configuração do verify vem da ref base

- Com `-c`: só o arquivo do `-c` e os padrões, exatamente como antes das réguas; o `.toolbelt/config.yaml` do repositório não é lido (nem na base, nem na árvore de trabalho).
- Sem `-c`: padrões < config global do usuário < o `.toolbelt/config.yaml` do repositório **como commitado na ref base** (`git show <baseSHA>:.toolbelt/config.yaml`). O arquivo da árvore de trabalho nunca é lido pelo verify. Isso vale para a seção `verify:` e para a seção `env:` (`allow`/`pass`), então uma mudança não consegue dar segredos aos próprios testes. O mesmo vale para o `.toolbelt/thresholds.yaml` (R4). Flags e argumentos MCP continuam sobrepondo, como hoje: vêm de quem chama, não da mudança.
- Quando a seção `verify:` ou `env:` da árvore de trabalho difere da do arquivo da base (edições não commitadas, um arquivo novo não commitado, ou edições commitadas no branch), o verify acrescenta um aviso com as chaves ignoradas: `ignored .toolbelt/config.yaml keys that differ from the base ref: verify.test_cmd (verify reads the base ref)`. O arquivo da árvore de trabalho comparado é o da raiz do repositório, também quando o verify roda de um subdiretório.
- Uma ref base que commita o `.toolbelt/config.yaml` como symlink (modo de árvore `120000`) não é seguida: `config_source: "none"` e o aviso `symlinked .toolbelt/config.yaml at the base ref is not followed; using the global config`. Um arquivo malformado na base aborta o verify (fail-closed). Com `-c` o arquivo do repositório não é lido, então nenhum dos dois se aplica.
- `overrides` (em ordem, `omitempty`) lista toda chave do verify cujo valor efetivo veio de uma flag ou argumento MCP e difere do que a config confiável (base/global/`-c`) definiu, mais os toggles de skip que desligaram uma etapa configurada (`test_cmd`, `no_typecheck`, `skip_checks:govulncheck`, `no_checks`). O modo texto (`tb verify run`, `tb stack`) imprime `Warning: overrides applied: …` no stderr.
- `config_source` só aparece fora do caso normal: omitido quando o arquivo da base foi usado (e com `-c`), `"none"` quando a ref base não tem o arquivo (só config global e padrões). Assim, execuções normais imprimem as mesmas chaves JSON de antes (G5).
- O guard compara a seção `verify:` do `.toolbelt/config.yaml` entre base e head. Uma diferença é um achado de erro (`verify_config_changed`) que aborta o verify como um acerto em caminho protegido, com as chaves alteradas listadas. As outras seções do arquivo continuam livres. Um symlink na base ou no head é comparado pelo texto do alvo; qualquer mudança é um achado que lista `verify` (não dá para fazer o parse). Um arquivo malformado na base é o achado de aviso `verify_config_unreadable`, para que o guard e execuções com `-c` sigam funcionando; um arquivo malformado no head continua sendo achado de erro.
- Quem chama escolhe o `--base`. Um agente pode commitar um limiar rebaixado ou um comando reescrito e passar `--base HEAD`. Por isso os docs dizem que o CI precisa fixar a base (o merge base do PR), e o verify avisa (`base resolves to head`) quando o SHA da base é igual ao SHA resolvido do head. Um `--base` (ou `base` no MCP) que aponta para dentro da mudança não é detectado além de base == head; o CI precisa fixar o merge base.
- Uma config global (ou arquivo do `-c`) malformada aborta o verify (fail-closed) em vez de ser ignorada em silêncio.
- Isso também fecha o buraco que já existe para `test_cmd`/`lint_cmd`/`build_cmd`/`hidden_test_cmd` e `env.pass`. Muda o comportamento de execuções cuja mudança edita essas chaves, e de usuários que editavam chaves de `verify:` ou `env:` sem commitar (agora ignoradas, com o aviso).

### R1. Seleção de régua

- `--ruler` (MCP `ruler`) aceita uma lista separada por vírgulas de `tests`, `coverage`, `visual`. Padrão `tests`.
- Um nome desconhecido falha antes de qualquer coisa rodar: `invalid ruler "x": tests, coverage or visual`, saindo com o código 2 de erro de uso (o main ganha um erro de uso tipado mapeado para 2; os demais erros continuam com 1). Hoje ele roda `tests` em silêncio; esta é uma correção deliberada.
- `visual` ainda não pode ser combinada com outras (ela ignora base/head e o guard): `visual` com outra régua é erro.
- Com várias réguas, o guard, a resolução de refs e a worktree do head rodam **uma vez**, num orquestrador, e cada régua recebe o mesmo diretório de trabalho (R6). Depois roda `tests`, depois `coverage`.
- Cada resultado de régua tem um `verdict` e, quando `SKIPPED`, um `skip_kind`: `not_applicable` (a régua não tem o que julgar ali, por exemplo coverage sem limiar em lugar nenhum) ou `unavailable` (deveria julgar mas não consegue, por exemplo um navegador ausente). Composição: o pior entre `ESCALATE` > `FAIL` > `SKIPPED(unavailable)` > `PASS`; uma régua `not_applicable` é ignorada, e se todas forem `not_applicable` o resultado é `SKIPPED(not_applicable)`. O `--dry-run` dá o plano de cada régua e o `dry_run: true` no topo, sem mesclar veredictos.
- Código de saída: 0 para `PASS` ou `SKIPPED`; senão, o primeiro código não zero na ordem das réguas (127 para comando ausente, 1 para o resto).

### R2. Passo `typecheck` (régua `tests`)

- Config nova `verify.typecheck_cmd`, flags `--typecheck-cmd` e `--no-typecheck`, MCP `typecheck_cmd` / `no_typecheck`, e o mesmo no `tb stack`.
- É um passo de gate, rodado depois do build e antes do lint, uma vez por execução.
- JSON: `typecheck_ran` e `typecheck_passed`, ambos `omitempty` (diferente de `lint_ran`, que está sempre presente, para que o G5 valha), e o trecho da falha em `reason`, como o lint faz hoje.
- Detecção no `tb init` (conservadora, só quando a ferramenta resolve no `PATH` ou pelo gerenciador de pacotes):
  - Node: um script `typecheck` vira `typecheck_cmd` (`<pm> run typecheck`) em vez de `build_cmd`; sem o script, um `tsconfig.json` e um `tsc` local dão o `tsc` pelo gerenciador de pacotes (`npm exec -- tsc --noEmit`, `pnpm exec tsc --noEmit`, `yarn tsc --noEmit`, `bunx tsc --noEmit`).
  - Python: `mypy` ou `pyright` configurado no `pyproject.toml` (`[tool.mypy]`, `[tool.pyright]`) ou nos arquivos de config deles, com a ferramenta no `PATH`. O fallback de lint atual ruff → mypy deixa de usar o mypy como lint.
  - Go e Rust: nada (o build já verifica tipos).
- Configs existentes continuam funcionando: um `build_cmd` que é um script de typecheck continua sendo build até o usuário movê-lo. Rodar o `tb init` de novo muda o que ele escreve para esses repositórios (`typecheck` sai de `build_cmd` e vai para `typecheck_cmd`; o mypy deixa de ser o fallback de lint do Python). Os testes afetados estão em `internal/config/detect_test.go` e `cmd/tb/init_json_test.go`, e o CHANGELOG registra isso.

### R3. Checks nomeados (régua `tests`)

- Lista nova na config:

  ```yaml
  verify:
    checks:
      - name: govulncheck          # [a-z0-9-]{1,32}, unique
        cmd: govulncheck ./...
      - name: clippy
        cmd: cargo clippy --all-targets -- -D warnings
  ```

- Cada check é um passo de gate, rodado depois do lint, em ordem, uma vez por execução. Saída 0 passa; qualquer outra reprova o check, o que termina o verify com `ESCALATE` como qualquer passo de gate.
- Flags: `--check name=cmd` (repetível, acrescenta ou sobrepõe um), `--skip-check name` (repetível; renomeada de `--no-check` na 1.0, que continua como alias oculto deprecated até a 2.0), `--no-checks`. MCP: `checks` (array de `{name, cmd}`), `skip_checks` (array de nomes) e `no_checks` (booleano, todos). O `tb stack` igual.
- JSON: `checks: [{name, passed, reason?}]`, `omitempty`.
- Um executável ausente falha rápido com saída 127, como os outros comandos. Na Fase A, um check não pode se marcar como não bloqueante nem "indisponível": um scan que precisa de rede (govulncheck) pertence ao CI, ou é pulado localmente com `--no-check` (Q1).
- O `tb init` sugere, nunca acrescenta em silêncio: com `--write-checks`, acrescenta `govulncheck` (Go, se estiver no `PATH`) e `clippy` (Rust, se `cargo clippy -V` funcionar). Sem a flag, imprime a sugestão.

### R4. Régua `coverage`

- **Medição:** a régua roda o `verify.coverage_cmd` na worktree do head e depois lê a cobertura total de linhas:
  - Go (padrão quando existe `go.mod` e `coverage_cmd` está vazio): `go test -coverprofile=<tmp> ./...`, depois o total ponderado por statements a partir do profile, calculado em Go (sem parsear o `go tool cover`).
    - Ponderação: cada bloco conta seus statements uma vez. Os blocos são deduplicados por `file:startLine.startCol,endLine.endCol` (um bloco que aparece várias vezes, como com `-coverpkg`, está coberto se qualquer cópia tiver contagem > 0). Total = statements cobertos / todos os statements, igual ao total do `go tool cover -func`.
    - Pacotes sem testes aparecem no profile com 0% (Go ≥ 1.22) e contam, como no `go tool cover`.
  - Genérico: `coverage_cmd` mais `verify.coverage_pattern`, uma expressão regular com exatamente um grupo de captura aplicada à saída do comando; o **último** match é a porcentagem, que precisa ser um número em [0, 100]. Os dois são validados ao carregar a config. Exemplos: `TOTAL.*\s(\d+(?:\.\d+)?)%` para o coverage.py (cobertura de linhas); `All files\s*\|\s*[\d.]+\s*\|\s*[\d.]+\s*\|\s*[\d.]+\s*\|\s*([\d.]+)` para a coluna `% Lines` do istanbul. A métrica é a que a ferramenta reporta (linhas na maioria, statements no Go), e o relatório diz qual é.
  - Sem toolchain e sem `coverage_cmd`: `SKIPPED(not_applicable)` quando também não há limiar; quando há limiar (arquivo da base ou `--min-coverage`), `FAIL` com `reason: "coverage: threshold set but no coverage_cmd for this repository"`, para que um piso configurado nunca passe em silêncio.
- **Limiar:** `coverage.min_percent` (um número em [0, 100]) nos **limiares base** (`.toolbelt/thresholds.yaml` lido da ref base com `git show <base>:.toolbelt/thresholds.yaml`, como o guard lê o `guard.yaml`).
  - A cópia da worktree nunca é lida: uma mudança que edita o arquivo já é bloqueada pelo guard, e ler da base torna a edição inútil mesmo com `--warn-only`.
  - O `--min-coverage N` (MCP `min_coverage`) só pode **subir** o limiar: o limiar efetivo é `max(base, N)`. Sem arquivo na base, `N` é o limiar.
  - Sem limiar nenhum: `SKIPPED(not_applicable)` com `reason: "coverage: no threshold"` (não rebaixa um `PASS` combinado, R1).
  - Um arquivo da base que não parseia, ou um `min_percent` fora de [0, 100], é erro (o verify aborta), como um `guard.yaml` malformado; nunca cai para "sem limiar". Schema: `version: 1`, `coverage: {min_percent: <number>}`; chaves desconhecidas são erro.
- **Veredicto:** `PASS` quando cobertura ≥ limiar, senão `FAIL` (não `ESCALATE`: o agente pode acrescentar testes). Um `coverage_cmd` que falha (testes falhando) é `FAIL` com o trecho.
- **JSON:** `coverage: {percent, threshold, threshold_source: "base"|"flag", cmd}` e `metrics["coverage_percent"]`.
- **Regras anti-trapaça:** o `.toolbelt/thresholds.yaml` continua sendo caminho protegido, e as chaves de `verify:` que escolhem os comandos são comparadas pelo guard (R0); o cenário "modify benchmark threshold" do ADR-005 se aplica aqui. Inflar cobertura sem asserções está fora do escopo (não-objetivo).

### R5. `tb stack` e MCP

- O `tb stack verify` ganha `--ruler` (mesma lista; `visual` é recusada ali), `--typecheck-cmd`, `--no-typecheck`, `--check`, `--no-check(s)`, `--coverage-cmd`, `--min-coverage`; o `tb_stack` do MCP, os mesmos campos.
- As três cópias da mesclagem com a config de arquivo (`cmd/tb/verify.go`, `parity.go`, `stack.go`) viram uma função em `internal/verify`, para que as chaves novas sejam lidas do mesmo jeito em todo lugar.

### R6. Forma interna

- Um orquestrador em `internal/verify` resolve as refs, carrega a config da base (R0), roda o guard e cria a worktree do head uma vez, e então chama cada régua com esse estado compartilhado: `Ruler` tem `Name()` e `Run(ctx, *RunState) (*RulerResult, error)`, em que `RunState` carrega a config, o diretório de trabalho e as refs. `tests`, `coverage` e `visual` a implementam (`visual` continua ignorando base/head, e é por isso que ainda não pode ser combinada).
- O `emitVerifyResult` imprime o payload de uma régua sem mudanças (G5) ou, para várias, `{verdict, rulers: {tests: …, coverage: …}}`.
- Os passos de gate compartilham um único executor (lint, typecheck e cada check), para que cache, `ESCALATE` e trechos se comportem igual.

## Critérios de aceitação

1. `tb verify run` com uma config existente e sem flags novas dá o mesmo veredicto e as mesmas chaves JSON de antes (teste golden num repositório fixture).
2. `--ruler foo` sai com 2 antes de rodar qualquer coisa; `--ruler visual,tests` é recusado.
3. Uma mudança que edita `verify.test_cmd` ou `verify.coverage_cmd` no `.toolbelt/config.yaml` é reportada pelo guard (`verify_config_changed`) e o verify aborta; o valor da base é o que rodaria. Uma edição não commitada (também com base == head) é ignorada com um aviso que cita as chaves; sem arquivo na base, o relatório diz `config_source: "none"`.
4. Um repositório cujos testes passam e cujo `typecheck_cmd` falha termina em `ESCALATE` com `typecheck_passed: false`; o typecheck roda uma vez entre iterações.
5. O `tb init` num fixture Node com script `typecheck` o escreve como `typecheck_cmd`, não `build_cmd`; com só `tsconfig.json` e um `tsc` local, escreve a forma do gerenciador de pacotes (`npm exec -- tsc --noEmit`, `pnpm exec tsc --noEmit`, `yarn tsc --noEmit`, `bunx tsc --noEmit`).
6. Dois checks, um falhando: `ESCALATE`, e `checks` lista os dois com o trecho do que falhou; `--no-check` pula um; um executável ausente sai com 127.
7. `--ruler coverage` num fixture Go calcula o mesmo total que o `go tool cover -func` (com diferença de até 0,1), também com duplicatas de `-coverpkg`; abaixo do limiar da base → `FAIL`; igual ou acima → `PASS`.
8. Uma mudança que baixa `coverage.min_percent` no `.toolbelt/thresholds.yaml` não baixa o limiar usado (a base é lida) e é reportada pelo guard.
9. `--min-coverage` abaixo do limiar da base não tem efeito; acima, sobe o limiar (`threshold_source: "flag"`).
10. `--ruler tests,coverage`: pior veredicto; sem limiar em lugar nenhum, um `tests` que passa dá `PASS`; com limiar mas sem comando de cobertura, `FAIL`.
11. `tb stack verify --ruler tests,coverage` e os `tb_verify_run` / `tb_stack` do MCP com os campos novos batem com a CLI.
12. O `coverage_pattern` genérico pega o último match; uma saída sem match é `FAIL` com `reason: "coverage: pattern did not match"`; um padrão sem exatamente um grupo é erro de config.
13. Um `thresholds.yaml` malformado na base aborta o verify; não é tratado como "sem limiar".
14. O `--dry-run` lista o typecheck, cada check e o comando de cobertura no plano; o formato JSON com várias réguas tem teste golden; os schemas MCP de `tb_verify_run` e `tb_stack` listam os campos novos (teste de paridade do registry).

## Docs a atualizar junto com a implementação

README (item do roadmap, linha do `tb verify`), `docs/cli.md` (flags do verify, tabela de réguas, tabela de detecção do init, flags do stack), ADR-008 (lista de réguas), CHANGELOG, e o espelho pt-BR de cada um.

## Questões em aberto

- Q1: um check deveria poder se declarar não bloqueante (`warn: true`) ou "indisponível" (sem rede), para que um scan reporte sem reprovar? Proposta: não na Fase A; usar `--no-check` localmente e o CI para o scan.
- Q2: cobertura de branches para Go (`-covermode=atomic` dá só statements). Proposta: só statements.
