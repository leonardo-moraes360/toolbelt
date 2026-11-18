🇧🇷 Português | [🇺🇸 English](../en/specs/egm-trust.md)

# Spec: EGM trust, Fase A.2 (fraude ativa do mesmo usuário)

Autor: toolbelt · Status: Rascunho, não revisado · Data: 2026-11-17 · Decisão: [ADR-014](../adr/014-egm-nativo.md) · Depende de: [núcleo](egm-nativo.md), spike R0

## Visão geral

O [núcleo](egm-nativo.md) assume um elephant honesto, mas desleixado. Esta spec reúne as proteções contra um elephant que **frauda ativamente** com shell como o mesmo usuário: um agente que tenta chegar a `PASS` sem revisão real editando config, estado, binários ou refs.

O conteúdo veio das rodadas 2 a 8 de revisão da versão monolítica da spec. Ali ficou claro que essa parte não converge enquanto o núcleo não estiver implementado e o spike R0 não trouxer dados reais. A meta desta fase é tornar a fraude **detectável e cara**, não impossível.

## Problema

Contra um processo do mesmo usuário, toda defesa local pode ser contornada, porque o atacante lê chaves, apaga estado, reescreve refs e forja TTY. Sem nenhuma defesa, porém, um agente consegue chegar a `PASS` em segundos, por exemplo:

- apontando `egm.agent_cmd` para um script que devolve atestações válidas;
- baixando `clean_rounds`;
- apagando o estado;
- escolhendo o range do gate.

## Objetivos

- T-G1: cada atalho de fraude conhecido deixa rastro verificável, ou exige um passo humano registrado.
- T-G2: o gate do `tb stack` não pode ser escolhido pelo elephant: nem o range, nem o documento, nem o conteúdo.
- T-G3: o limite declarado é completo, ou seja, tudo o que continua possível está listado.

## Não-objetivos

- Impedir fraude de um processo do mesmo usuário (é impossível localmente).
- Rodar sob outro usuário ou numa VM (fica para depois).
- Implementar os tokens de capacidade do [ADR-005](../adr/005-anti-trapaca.md) §4.

## Plano técnico

Os mecanismos abaixo são candidatos. Cada um virou requisito na versão monolítica e foi revisado pelo menos uma vez. A tabela resume o desenho e os problemas que ficaram abertos.

| Mecanismo | Resumo | Problemas já conhecidos |
|-----------|--------|-------------------------|
| Raiz de usuário | Config de usuário, `<state>` e âncoras resolvidos por `os/user.Current().HomeDir`, ignorando `XDG_*`/`HOME`, com o aviso `xdg_ignored`; `-c` ignorado para chaves sensíveis | Diverge de `config.GlobalConfigPath()`; afeta usuários com `XDG_*` legítimo |
| Classes de chave | Execução e segredo só na config de usuário (já no núcleo); política do ref de confiança via `cat-file blob`; `config_untrusted` por divergência | Ref de confiança local é reescrevível; fallback para o `main` local seleciona range vazio quando o commit é direto no `main` |
| Agente fixado | sha256 do binário, do interpretador (incluindo `/usr/bin/env X`) e das árvores de runtime; canário de ferramenta no `pin`; `agent_cmd` diferente do binário do tb | TOCTOU entre a verificação e o `exec`; hash de árvore custa I/O a cada execução; atualizações do vendor exigem `pin` |
| Build do tb | `debug.ReadBuildInfo` por rodada; build local rebaixa salvo `trust_dev_builds` | Atrapalha o dogfooding deste repositório |
| Cadeia e âncora | `hash = sha256(prev ‖ registro)`, `audit.jsonl` append-only, âncora separada por `repo-id`, `repair` humano | Um crash entre o audit e o estado bifurca a cadeia, então é preciso WAL em lote; forks e clones compartilham o `repo-id` e geram falso `trust_ref_rewound` |
| Confirmação humana | TTY (`os.ModeCharDevice`) para `--force-settle`, `--restart`, `--narrow`, `--past-cap`, `--decline` de major, `pin`, `repair` e rewind | TTY forjável; bloqueia fluxos 100% agênticos |
| Gate por ref de confiança | Documentos selecionados por `merge-base(trust_sha, head)..head`, independente de `--base` | Seleção vazia, ou indefinida sem merge-base (clone raso) |
| Séries por branch | Contadores de teto e orçamento por branch | Identidade de série ambígua com `--restart` |
| `internal/gitsafe` | Unificar o endurecimento de git de router, merge e egm | A união das listas leva `diff.ignoreSubmodules=all` ao merge e esconde gitlinks na checagem de caminhos sensíveis; o env sem `GIT_CONFIG_*` quebra o `withGitNoDetach` do router e o `hooksDirs` do merge |

## Limite declarado (rascunho)

Mesmo com tudo acima, um processo do mesmo usuário pode fazer qualquer uma destas coisas:

- editar a config de usuário;
- reescrever refs locais e fazer commits;
- apagar ou editar `<state>`;
- fazer um clone raso;
- renomear o documento para recomeçar;
- forjar um TTY;
- trocar arquivos fixados entre a verificação e o `exec`;
- rodar um `tb` modificado;
- montar um `.git/config` hostil num repositório que ele mesmo criou.

## Questões em aberto

- Vale a pena manter cada mecanismo, dado que todos são contornáveis localmente? O spike R0 e o uso real do núcleo devem dizer quais atalhos aparecem na prática.
- O gate estrito deveria exigir execução em outro usuário, numa VM ou em CI com credenciais próprias, em vez de defesas locais?

## Plano de Implementação Detalhado

Ainda não definido. Esta spec só segue para revisão depois que o núcleo convergir e o R0 for concluído.

*Elaborado com base no padrão EGM*
