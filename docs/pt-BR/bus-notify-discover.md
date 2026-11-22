[🇺🇸 English](../bus-notify-discover.md) | 🇧🇷 Português

# Adapter `notify` do barramento e `tb bus discover` — spec do restante do B3

Status: **implementado** (restante do B3).

Fecha a etapa **B3** do [ADR-013](adr/013-barramento-multi-vendor.md). Estende o harness ([bus-harness.md](bus-harness.md)), cujo primeiro corte entregou os adapters `exec` e `harness`, e se apoia no lease do B2 ([bus-lease.md](bus-lease.md)).

## Problema

- **Agentes só de GUI.** Um agente que vive num app desktop (um cliente MCP sem modo headless) não pode ser acordado por um processo. Hoje o humano só descobre que ele tem trabalho rodando `tb bus log`, ou ligando o `exec` ao `notify-send` na mão.
- **Onboarding.** Antes de escrever um arquivo de wake o humano precisa saber quais agentes a máquina tem, quais o `tb` consegue acordar e quais já apontam uma config MCP para o `tb`. O ADR-013 chama isso de introspecção: um catálogo que diz o que é possível, e não é consentimento nem canal.

## Princípios

1. **Quem é acordado é o humano.** O `notify` avisa o humano, nunca o agente. O agente continua lendo com `recv` quando o humano abre um turno.
2. **Nada de conteúdo na notificação.** Ela leva só contagens, tipos de mensagem e nomes de remetente, nunca payload, thread ou qualquer outro texto livre.
3. **A descoberta lê, nunca age.** O `discover` não cria convite, não grava arquivo, não entra em canal, não abre conexão e não executa binário.
4. **Nada de segredo no catálogo.** As configs MCP são lidas só pela forma da entrada do `tb mcp`; comandos, argumentos e ambiente de outros servidores nunca saem do processo, e erros de parse não são impressos.
5. **Só CLI.** Nem as configurações de notificação nem o catálogo ficam expostos no MCP: o catálogo descreve a máquina do humano, e um agente não tem uso para ele.

## Parte 1: adapter `notify`

### Onboarding de um agente de GUI

O `tb bus run` precisa da sessão CLI do agente (`<private>/sessions/<channel>/<name>/`), que só o `tb bus join` grava. Então um agente de GUI entra pela CLI e o cliente dele é ligado a essa sessão:

1. O humano roda `tb bus join <channel> --invite C --name <name> --wake notify` (ou `tb bus run --invite C ...`).
2. O humano aponta a config MCP do cliente de GUI para `tb mcp --bus-as <channel>/<name> --bus-root <root>`, a mesma ligação que o adapter `harness` usa.
3. O agente nunca chama `tb_bus_join`: um segundo join trocaria o token e invalidaria a sessão do harness. Com `--bus-as` a conexão já está ligada, e o `tb_bus_join` nela falha com `already in channel`.

### Configuração

O arquivo de wake (`<private>/wake/<channel>/<name>.yaml`, `0600`) ganha `adapter: notify`:

```yaml
adapter: notify
notify:
  backend: desktop         # desktop | terminal
  command: ""              # só desktop: caminho absoluto; vazio = o primeiro da lista fixa abaixo
  urgency: normal          # low | normal | critical
  urgent_urgency: critical # urgência para lotes com mensagem urgent; low | normal | critical
rewake_after: 1h           # padrão para notify (10m nos outros adapters)
timeout: 10s               # padrão para notify (30s no exec)
```

- Os campos comuns `wake_on`, `wake_topics`, `coalesce` e `budget` mantêm o sentido.
- Estes são erros do `Validate`, então tanto o `tb bus run` quanto o `--check` os recusam:
  - `pass_env`, `secret_files`, `sandbox.read` ou `sandbox.write` não vazios com `adapter: notify` (o backend não precisa de nenhum deles);
  - um bloco `exec:` ou `harness:` com `adapter: notify`, e um bloco `notify:` com outro adapter;
  - `notify.backend` ausente (é obrigatório) ou `backend: desktop` numa plataforma que não seja Linux nem macOS (`notify_unsupported`).
- Misturar blocos `exec:` e `harness:` continua aceito, como antes do `notify` existir.
- Os padrões dependem do adapter: `rewake_after` é `1h` e `timeout` é `10s` para `notify`.

### Backends

| Backend | Plataforma | Roda |
|---|---|---|
| `desktop` | Linux | `<command> --app-name toolbelt --urgency <u> -- <resumo> <corpo>` |
| `desktop` | macOS | `/usr/bin/osascript -e 'on run argv' -e 'display notification (item 1 of argv) with title (item 2 of argv)' -e 'end run' <corpo> <resumo>` (sem `--`: o corpo começa com uma contagem, nunca com `-`) |
| `terminal` | qualquer | sem filho: uma linha escrita pelo próprio harness |

**Binário do desktop.**

- `command` vazio quer dizer o primeiro arquivo existente entre `/usr/bin/notify-send` e `/usr/local/bin/notify-send`. Não há busca no `PATH`; no macOS o caminho é sempre `/usr/bin/osascript`.
- O caminho é resolvido uma vez, quando o harness abre (seguindo symlinks), e é esse arquivo que todo wake roda.
- Quando o harness abre (então tanto o `tb bus run` quanto o `--check` falham), o binário é recusado quando:
  - ele, ou qualquer diretório acima dele até `/`, não pertence ao root ou tem escrita para grupo ou outros (um diretório sticky do root, como `/tmp`, é aceito). No NixOS o `/nix/store` tem escrita para o grupo, então defina `command` com um caminho que passe, ou use `backend: terminal`;
  - ele fica dentro da raiz do repositório ou de `<private>`.
  
  Um agente com acesso a shell consegue gravar `~/.local/bin/notify-send`; não consegue trocar um binário de sistema do root.
- O texto entra como itens do `argv`, nunca por um shell, e nunca dentro do código AppleScript.

**Sandbox e seu limite.**

- O filho roda no sandbox Landlock do `tb` como os outros adapters. Ele lê os diretórios do sistema e o diretório do próprio executável, e não escreve nada além do workdir padrão.
- `XDG_RUNTIME_DIR` não ganha permissão: o Landlock (até a ABI 5) não controla `connect()` em sockets unix, então o socket do D-Bus funciona sem ela.
- Sem Landlock vale a regra dos outros adapters: recusado, a menos que `--allow-unsandboxed`.
- O barramento de sessão é uma capacidade que o Landlock não limita: um processo nele pode pedir ao gerenciador de serviços do usuário que inicie comandos fora do sandbox. Isso é aceito para o `notify` porque o binário é um arquivo de sistema do root que o `tb` resolveu, e a entrada dele é só o texto validado abaixo. O sandbox protege contra acesso a arquivos por esse binário, não contra um binário hostil; é a checagem de dono que mantém um binário hostil de fora.

**Ambiente.** O filho do desktop recebe só:

- `PATH`, `HOME`, `LANG` e `LC_*`;
- quando definidos, `DBUS_SESSION_BUS_ADDRESS` e `XDG_RUNTIME_DIR`.

`DISPLAY`, `WAYLAND_DISPLAY`, as variáveis `TB_BUS_*` do `exec` e todo o resto ficam de fora.

**Terminal.** O harness escreve a linha ele mesmo, no próprio stderr (um writer injetável no código), seguida do sino (`\a`). Não há filho, então não há checagem de sandbox nem `--allow-unsandboxed`. A escrita nunca faz o wake falhar, a menos que o stderr esteja fechado.

### Texto

As strings são saída fixa em inglês em qualquer locale:

```
summary: toolbelt: marin has 3 new messages in feature-x
body:    2 task.assign, 1 query from "claude-code". Open marin and ask it to call tb_bus_recv. Peek from a shell: tb bus recv --channel feature-x --as marin --peek
```

(Em português: "marin tem 3 mensagens novas em feature-x"; "2 task.assign, 1 query de claude-code. Abra marin e peça para chamar tb_bus_recv. Para espiar de um shell: ...".)

- **Só valores limitados.**
  - Nomes de canal e de agente casam com `ValidName` (`[a-z0-9][a-z0-9._-]`, no máximo 63 caracteres).
  - Os tipos vêm do enum fechado `Type`.
  - Nenhuma thread, payload, `causation_id` ou chave de idempotência entra.
- **Nomes de remetente são escolhidos pelo agente.** Um convite sem nome vinculado deixa quem entra escolher qualquer nome válido, então um nome pode parecer uma instrução. Por isso os remetentes aparecem entre aspas, sem o prefixo `agent:` e cada um cortado em 32 caracteres com `…`. `system:watchdog` e qualquer remetente `system:*` aparecem como `the broker`.
- No máximo 3 remetentes, ordenados por contagem e depois por nome, seguidos de `and 1 other` ou `and N others`. Todos os remetentes `system:*` contam como um só `the broker`.
- Um `rewake` troca `new` por `still unread`. Quando o lote tem uma mensagem `urgent`, o resumo começa com `urgent: ` e a urgência é `urgent_urgency`. Qualquer membro pode mandar `urgent`, então no GNOME e no KDE `critical` fura o Não Perturbe; use `urgent_urgency: normal` para evitar isso. O orçamento limita isso a `max_wakes` por `per` de qualquer forma.
- **A dica de shell usa `--peek`.** Um `recv` comum avançaria o cursor compartilhado do agente, e o `tb_bus_recv` do agente de GUI não devolveria mais nada.

### Contagens no packet

A lista `messages` do packet de wake para em 20 itens, então o texto não pode ser montado a partir dela. O `bus-wake/0.1` ganha dois campos aditivos, preenchidos para todo adapter a partir de todas as mensagens do lote:

```json
"types": {"query": 1, "task.assign": 2},
"senders": [{"from": "agent:claude-code", "count": 3}]
```

`senders` é ordenado por contagem decrescente, depois por `from`, e é limitado a 20 entradas mais `"senders_more": N`. O adapter `exec` também os recebe; consumidores que não os conhecem os ignoram.

### Semântica de entrega

- Uma notificação é um wake: coalescência, orçamento, retentativas, `wake_failed` e a auditoria funcionam como em [bus-harness.md](bus-harness.md). O orçamento padrão continua `6` por `1h`, o que também limita quantas vezes o humano é chamado.
- **Exit 0 quer dizer que o texto foi entregue ao sistema operacional, não que o humano viu.** Por exemplo, o macOS descarta notificações em silêncio quando elas estão desativadas para o app que chama, e não existe sessão de GUI sob `launchd` ou SSH. A única confirmação real é o cursor do agente:
  - depois de `rewake_after` (padrão `1h` aqui, já que um humano pode demorar), um packet não lido ganha um lembrete (`reason: rewake`);
  - se continuar não lido, o harness registra `wake_unread`.
- O evento de auditoria ganha um campo `backend` (`desktop` ou `terminal`) em `wake_start`, `wake_ok` e nos eventos de falha. O texto da notificação nunca é gravado.

### Agent Card e avisos

- `wake: notify` já existe como valor no card.
- Quando o arquivo de wake diz `adapter: notify` e o card do agente diz outro `wake`, tanto o `--check` quanto a abertura do `tb bus run` imprimem `warning: wake_mismatch (card says harness, wake file says notify)` no stderr e seguem com exit 0. O card é informativo para o canal; quem manda é o arquivo de wake. O aviso também diz como corrigir: fazer o card dizer `notify` (parar o `tb bus run`, `tb bus invite <canal> --role <papel> --name <nome>` e depois `tb bus join <canal> --invite <código> --name <nome> --wake notify`) ou mudar `adapter` no arquivo de wake, cujo caminho ele imprime.
- Os avisos vêm de uma chamada nova `Warnings()` ao lado do `Check()`; a CLI os imprime nos dois casos.

## Parte 2: `tb bus discover`

### Comando

```bash
tb bus discover
tb bus discover --json
tb bus discover --cwd ~/src/app   # lê o .mcp.json desse diretório em vez do da raiz do repositório
```

O `discover` não precisa de `.toolbelt/` nem de barramento. Sem `--cwd`, ele lê o `.mcp.json` do repositório git que contém o diretório atual. Fora de um repositório, a seção do projeto informa `"project": null`.

### O que inspeciona

**1. CLIs no `PATH`**, a partir de uma tabela embutida de CLIs de agente conhecidas. A tabela vive no código e nesta spec. Uma CLI fora dela nunca é informada, e acrescentar uma linha é mudança de código.

| CLI | Vendor | Headless/resume | Preset do `tb` | Wake sugerido |
|---|---|---|---|---|
| `claude` | anthropic | sim | `claude` | `harness` |
| `codex` | openai | sim | ainda não | `exec` |
| `gemini` | google | sim | ainda não | `exec` |
| `opencode` | sst | sim | ainda não | `exec` |
| `aider` | aider | não | nenhum | `manual` |

- Cada entrada do `PATH` é consultada em ordem. Entradas relativas ou vazias são puladas e listadas em `skipped_path_entries`, porque dependem do diretório atual.
- Para cada CLI encontrada: o caminho absoluto, o alvo do symlink e se o arquivo tem escrita para alguém além do dono (`writable_by_others`). Uma dica, não uma recusa.
- Nada é executado. Não há sondagem de versão no B3: muitas CLIs de agente acessam a rede no `--version`.

**2. Configs de cliente MCP**, nestes caminhos:

| `client` | Caminho | Lido |
|---|---|---|
| `claude-code` | `~/.claude.json` | `mcpServers` no nível de cima (`scope: user`) e em cada `projects.<path>` que tenha ao menos um servidor |
| `claude-desktop` | `~/.config/Claude/claude_desktop_config.json` (Linux), `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) | `mcpServers` |
| `cursor` | `~/.cursor/mcp.json` | `mcpServers` |
| `project` | `<cwd ou raiz do repo>/.mcp.json` | `mcpServers` |
| `codex` | `~/.codex/config.toml` | só existência (`parsed: false`, sem dependência de TOML no B3) |

**Leitura segura:**

- `Lstat` primeiro: symlink, FIFO, dispositivo ou diretório é informado como `"kind": "symlink" | "other"` e nunca aberto.
- Arquivos regulares são abertos com `O_NOFOLLOW|O_NONBLOCK`, conferidos de novo com `fstat` e recusados acima de 4 MiB (`too_large`).
- Um erro de parse informa `"parses": false` e nada mais: a mensagem do parser, que pode citar bytes do arquivo, é descartada.

**Como casar a entrada do `tb mcp`.** Uma entrada de servidor conta como `tb` quando valem as duas condições:

- `command` é um caminho absoluto cujo basename é `tb`, ou o nome puro `tb`;
- `args[0] == "mcp"`.

Um `tb` puro é informado como `"path": "tb"` com `"same_tb": null`, porque ele é resolvido no `PATH` do cliente, não no nosso. Wrappers de shell (`sh -c ...`, `npx`, `env ...`) não são interpretados: contam como `"other"`.

**O que é informado por entrada:**

- para a entrada casada: a chave do servidor, o caminho absoluto, se é o mesmo arquivo do `tb` em execução (`os.SameFile`) e se `args` tem `--bus-as` (o valor só é impresso quando é um par canal/nome válido; senão aparece como `(invalid)`);
- a saída em texto põe entre aspas qualquer caminho ou chave de projeto com caracteres de controle ou não imprimíveis, para que o `.mcp.json` de um repositório clonado não mande sequências de escape ao terminal;
- `--bus-root`, os outros argumentos e o `env` da entrada `tb` nunca são impressos;
- outros servidores só são contados (`"other_servers": N`).

### Saída

O `--json` imprime o schema abaixo: chaves estáveis, arrays ordenados por nome ou caminho, `~` nunca aparece na saída (só caminhos absolutos).

```json
{
  "v": "bus-discover/0.1",
  "tb": "/home/leonardo/go/bin/tb",
  "skipped_path_entries": ["node_modules/.bin"],
  "clis": [
    {"name": "claude", "vendor": "anthropic", "path": "/home/leonardo/.local/bin/claude",
     "target": "/home/leonardo/.local/share/claude/versions/2.1.0", "writable_by_others": false,
     "headless": true, "preset": "claude", "wake": "harness"}
  ],
  "mcp_configs": [
    {"client": "claude-code", "path": "/home/leonardo/.claude.json", "kind": "file", "parses": true,
     "scope": "user", "tb": [{"server": "toolbelt", "path": "/home/leonardo/go/bin/tb", "same_tb": true, "bus_as": null}],
     "other_servers": 2},
    {"client": "claude-code", "path": "/home/leonardo/.claude.json", "kind": "file", "parses": true,
     "scope": "project:/home/leonardo/src/app", "tb": [], "other_servers": 1},
    {"client": "claude-desktop", "path": "/home/leonardo/.config/Claude/claude_desktop_config.json", "kind": "missing"},
    {"client": "codex", "path": "/home/leonardo/.codex/config.toml", "kind": "file", "parsed": false}
  ],
  "project": "/home/leonardo/src/app"
}
```

- `kind` é um de `file`, `missing`, `symlink`, `other` ou `too_large`.
- `parses` só aparece quando `kind` é `file`.

O texto imprime os mesmos dados em três grupos (CLIs, configs MCP, próximos passos). Os próximos passos são comandos para copiar, nunca rodados:

```
next (CLI agents):  tb bus invite <channel> --role worker --name <name>
                    tb bus run --channel <channel> --as <name> --invite <code>   (write the wake file first, see docs/bus-harness.md)
next (GUI agents):  tb bus join <channel> --invite <code> --name <name> --wake notify
                    point the client at: tb mcp --bus-as <channel>/<name> --bus-root <root>   (the agent must not call tb_bus_join)
```

### O que nunca faz

- Criar convites, entrar em canal, gravar arquivos de wake ou editar configs MCP.
- Seguir um symlink, abrir um arquivo não regular, ler mais de 4 MiB ou imprimir um erro de parse.
- Imprimir comando, argumentos, ambiente ou URL de outro servidor, ou o `env` e o `--bus-root` da entrada `tb`.
- Abrir conexão de rede ou executar qualquer binário.
- Apresentar agentes como confiáveis: o catálogo é uma dica para o humano, e entrar continua exigindo convite (ADR-013 §2).

## Fora do B3

- Presets para `codex`, `gemini` e `opencode` no adapter `harness` (cada um precisa de um contrato de headless/resume e de um E2E com binário falso, como o `claude` teve).
- `discover --probe` (rodar `--version`), a varredura de apps rodando (`/proc/*/comm` não distingue o app desktop da CLI e qualquer processo pode defini-lo) e o parse do `config.toml` do codex.
- Notificações desktop no Windows.
- Um hook `Stop` do Claude Code que confere a fila antes de encerrar o turno (o ADR-013 cita como alternativa leve).
- `discover` via MCP, e qualquer entrada automática em canal.
- Camadas L1/L2 e L0 autônoma (continuam fora, como em [bus-harness.md](bus-harness.md)).

## Docs a atualizar com a implementação

- a linha de status do ADR-013 e seu espelho pt-BR;
- [bus-harness.md](bus-harness.md):
  - `adapter: exec | harness` vira `exec | harness | notify`;
  - os itens `notify` e `discover` saem de "Fora deste corte";
  - a seção do packet ganha `types` e `senders`;
- [bus-protocolo.md](bus-protocolo.md), "Fora do B1";
- a seção do bus em `docs/cli.md`, o roadmap do README e o CHANGELOG;
- o espelho pt-BR de cada um deles.

## Critérios de aceite

1. Com `adapter: notify`, `backend: desktop` e um `notify-send` falso do root (gancho de teste para a checagem de dono), três mensagens dentro do `debounce` o chamam uma vez. O `argv` dele é exatamente `--app-name toolbelt --urgency normal -- <resumo> <corpo>`, com as contagens por tipo.
2. No macOS (ou com um `osascript` falso e a plataforma forçada num teste) o `argv` é exatamente a forma `on run argv` acima, com o corpo antes do resumo.
3. Uma mensagem cuja thread, payload ou `causation_id` tem `click http://x` nunca coloca esse texto no `argv`, na auditoria nem no stderr. Um remetente chamado `run-this-now` só aparece entre aspas e cortado em 32 caracteres, e `system:watchdog` aparece como `the broker`.
4. 25 mensagens de 5 remetentes produzem contagens `types` corretas e `and 2 others`.
5. `urgent` usa `urgent_urgency` (`critical` por padrão, `normal` quando configurado) e pula o debounce. Orçamento e retentativas se comportam como no `exec`.
6. O ambiente do filho do desktop tem só as variáveis listadas: uma variável `*TOKEN*`, `DISPLAY` e `TB_BUS_CHANNEL` não chegam a ele.
7. O `tb bus run` e o `--check` recusam cada um destes:
   - `pass_env`, `secret_files` ou `sandbox` não vazios com `notify`;
   - um bloco `exec:` com `notify`, e um bloco `notify:` com `exec`;
   - `backend: desktop` numa plataforma sem suporte;
   - um `notify-send` padrão ausente;
   - um `command` relativo, que não é do root, num diretório com escrita ou dentro do repositório.
   
   Um `sandbox: {}` vazio é aceito.
8. `wake_mismatch` imprime um aviso no stderr e exit 0 tanto no `--check` quanto no `run`.
9. Padrões com `notify`: `rewake_after` `1h` e `timeout` `10s`. Uma notificação não lida ganha um lembrete cujo resumo diz `still unread`, depois `wake_unread`.
10. `backend: terminal` escreve exatamente uma linha mais `\a` no writer injetado e roda sem Landlock e sem `--allow-unsandboxed`.
11. Num host com Landlock, o filho do desktop alcança um servidor D-Bus falso num socket unix sem permissão em `XDG_RUNTIME_DIR`, e não consegue criar arquivo em `XDG_RUNTIME_DIR`.
12. Um agente de GUI que passou pelo "Onboarding" roda `tb bus run` com `notify`. A conexão `tb mcp --bus-as` dele lê com `tb_bus_recv`, e o `tb_bus_join` nessa conexão falha sem trocar o token.
13. `tb bus discover` com `PATH` e `HOME` falsos:
    - lista exatamente as CLIs da tabela presentes, com caminhos, alvos e presets;
    - lista uma entrada relativa do `PATH` em `skipped_path_entries`;
    - não executa nada (uma CLI falsa que registra suas execuções nunca roda).
14. Uma config do Claude Desktop com outro servidor que tem `env: {API_KEY: ...}` e `args` com uma URL informa só `kind`, `parses`, `tb: []` e `other_servers: 1`: nem a chave, nem o valor, nem a URL aparecem no texto ou no JSON.
15. Um `.mcp.json` que é symlink para um arquivo secreto, uma FIFO, um arquivo de 5 MiB ou um JSON inválido que cita um segredo rende, nessa ordem, `symlink`, `other`, `too_large` e `parses: false`. O `discover` não trava, e nenhum conteúdo de arquivo aparece.
16. Uma entrada `tb` que aponta para outro binário informa `same_tb: false` com esse caminho. Um `tb` puro informa `same_tb: null`, e `sh -c "tb mcp"` conta em `other_servers`. As entradas por projeto do `~/.claude.json` são informadas por caminho de projeto.
17. Fora de um repositório git e sem `--cwd`, `project` é `null` e nada falha.
18. Nenhuma tool MCP expõe o `discover` nem as configurações de notificação.
