# Landing page (GitHub Pages)

O site estático em [`site/`](../site/) é a landing pública do toolbelt (`tb`).

URL prevista: <https://leonardo-moraes360.github.io/toolbelt/>

## Conteúdo

- HTML/CSS/JS mínimo, sem framework nem build step
- Caminhos relativos (`./styles.css`, `./assets/…`) para funcionar no subpath `/toolbelt/`
- Tema Synthwave alinhado a [`identidade-visual.md`](identidade-visual.md)
- Toggle PT/EN opcional (pt-BR é o idioma principal)

## Publicação

O workflow [`.github/workflows/pages.yml`](../.github/workflows/pages.yml) publica o diretório `site/` via GitHub Actions:

- dispara em `push` para `main` que altere `site/**` ou o próprio workflow
- também via `workflow_dispatch`
- permissões mínimas: `contents: read`, `pages: write`, `id-token: write`
- concurrency group `pages`
- actions pinadas por versão (`configure-pages`, `upload-pages-artifact`, `deploy-pages`)

### Habilitar Pages no repositório

O Pages **ainda precisa ser habilitado manualmente** por um admin do repositório. O workflow **não** usa `enablement: true`.

1. Abra **Settings → Pages**
2. Em **Build and deployment → Source**, escolha **GitHub Actions**
3. Faça merge na `main` (ou rode o workflow manualmente) após o site existir em `site/`

Até essa configuração, o workflow pode falhar no job de deploy por ambiente `github-pages` ausente — isso é esperado.

## Pré-visualização local

```bash
cd site
python3 -m http.server 8080
# abra http://127.0.0.1:8080/
```

Verifique links internos (`#problema`, `#comandos`, `#mcp`, `#instalar`, `#docs`), o botão Copiar nos blocos de comando e o layout em viewport estreita.
