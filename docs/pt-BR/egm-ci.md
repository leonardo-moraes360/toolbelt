[🇺🇸 English](../egm-ci.md) | 🇧🇷 Português

# Gate do EGM no CI

O resultado local do `tb egm` é autodeclarado: o processo que está sendo julgado pode editar o próprio estado, a config e o agente. No CI, `tb stack egm --egm-mode ci` confere o **recibo** commitado de cada documento sob gate contra o commit e depois roda **uma rodada nova de revisão** com a config e a credencial do próprio CI. O desenho e os limites estão na [spec de trust](specs/egm-trust.md). A referência do comando está em [`docs/cli.md`](cli.md).

## 1. Fluxo do autor

Para cada spec ou ADR (`egm.doc_globs`, padrão `docs/specs/**` e `docs/adr/**`):

```bash
tb egm review docs/specs/x.md            # depois edite, resolva e rode o loop até convergir
tb egm loop next docs/specs/x.md
git commit -am "docs: x"                   # o documento e as fontes primeiro
tb egm receipt docs/specs/x.md            # .toolbelt/egm/receipts/<slug>.json
git add .toolbelt/egm/receipts && git commit -m "docs: egm receipt for x"
```

O `tb egm receipt` recusa se o veredito não for `PASS` ou `PASS_PARTIAL`, e recusa com `receipt_uncommitted` quando o documento ou uma fonte difere de `HEAD`. Rodar `tb stack egm` localmente avisa `receipt_missing` ou `receipt_stale` antes do push.

## 2. O workflow

Copie [`docs/examples/egm-ci.yml`](../examples/egm-ci.yml) para `.github/workflows/egm.yml` e:

- ajuste `TB_VERSION` para uma release que tenha `--egm-mode ci`, e `TB_SHA256` para a linha desse arquivo no `checksums.txt` da release ([docs/release.md](release.md));
- fixe `CLAUDE_CODE_VERSION`;
- crie o secret de repositório `ANTHROPIC_API_KEY`.

O workflow escreve a config do egm do CI em `$RUNNER_TEMP` (nunca a partir do repositório), passa os SHAs de base e head do PR e roda em todo `pull_request`, sem filtro de `paths`. Um PR que não toca nenhum documento sob gate passa com o aviso `no_gated_docs`.

O exemplo começa com `continue-on-error: true`. **Enquanto ele estiver lá o job passa mesmo quando o gate reprova, então o check é só consultivo e torná-lo obrigatório não protege nada.** Mantenha até medir com que frequência uma rodada nova acha um `major` novo em documentos que convergiram localmente (spec, Questões em aberto); depois tire.

## 3. Configurações do repositório

A verificação vale o quanto vale a proteção do workflow. No modo ci os globs do gate são a união dos do merge-base e dos de head, então um PR não tira um documento do gate editando o `.toolbelt/config.yaml`. Sem estas configurações, o agente revisado consegue editar o workflow no próprio PR.

- **Status check obrigatório:** torne o job `egm` obrigatório na branch padrão.
- **Proteção de branch:** exija pull requests e o status check, com **"include administrators"** (ou o ruleset equivalente sem bypass).
- **CODEOWNERS**, com pelo menos um dono que não seja o autor dos PRs revisados:

  ```text
  /.github/CODEOWNERS        @owner-a @owner-b
  /.github/workflows/**      @owner-a @owner-b
  /.toolbelt/config.yaml     @owner-a @owner-b
  ```

  Ligue "Require review from Code Owners". **Não** coloque `.toolbelt/egm/receipts/**` sob CODEOWNERS: os autores atualizam os recibos a cada mudança, e o CI confere de qualquer jeito.

## 4. O que reprova, e o que fazer

| Resultado | Significado | O que fazer |
|-----------|-------------|-------------|
| `no_receipt`, `receipt_invalid`, `receipt_stale` | O recibo falta, está malformado ou é mais antigo que o documento ou as fontes | Convergir, commitar, `tb egm receipt`, commitar |
| `ci_finding` | A rodada nova achou um `major` ou `blocker` que o recibo não explica | Corrigir localmente e atualizar o recibo |
| `ci_downgrade` | O finding casa com uma alegação que o autor recusou ou encerrou à força | Um humano decide: corrigir, ou fazer o merge pelo mecanismo de exceção do repositório |
| `ci_upheld` | O árbitro do CI manteve um finding que o recibo diz ter sido retirado pelo árbitro | Corrigir localmente |
| `ci_partial`, `ci_not_run` | A rodada nova foi parcial (um downgrade, um revisor que não completou) | Ver o log do job; rodar de novo se foi passageiro. Todos os revisores `not_run` costuma ser `ANTHROPIC_API_KEY` errada ou revogada: confira o secret |
| `ci_offline` | Sem credencial (um PR de fork não recebe secrets, ou a chave está vazia) | Rodar o job a partir de uma branch do repositório, ou aceitar `--egm-allow-offline` e o teto `PASS_PARTIAL` |
| `ci_budget` | Mais documentos que `egm.ci.max_docs`, ou o passo passou de `egm.max_agent_runs` | Dividir o PR, ou subir os limites na config do CI |
| `gated_doc_removed` | Um documento sob gate foi apagado ou movido para fora do gate | Um humano revisa a remoção |

## Limites

- A rodada nova é **uma amostra só**: um `major` que o loop local e uma rodada do CI deixam passar entra no merge.
- Num repositório em que o único dono de código é também o admin cujo token `gh` o agente pode usar, o agente ainda consegue mudar a proteção ou fazer o merge direto. O tb não detecta a falta de CODEOWNERS nem de proteção de branch.
- O exemplo ainda não rodou num runner hospedado. Ele assume Landlock (os kernels do Ubuntu 24.04 o habilitam) e que o claude do npm roda com o prefixo do node como único caminho extra de leitura.
- Cadeia de suprimentos: o arquivo do tb é conferido só por sha256 (a assinatura cosign do `checksums.txt` não é verificada), e o npm instala o claude sem pin de integridade. Fixe `actions/checkout` num SHA de commit se a sua política exigir.
- Custo: uma rodada por documento selecionado (uma execução de agente por revisor obrigatório), mais no máximo uma rodada do árbitro por alegação arbitrada de novo.
