🇺🇸 English | [🇧🇷 Português](pt-BR/site.md)

# Landing page (GitHub Pages)

The static site in [`site/`](../site/) is toolbelt's public landing page (`tb`).

Expected URL: <https://leonardo-moraes360.github.io/toolbelt/>

## Content

- Minimal HTML/CSS/JS, no framework or build step
- Relative paths (`./styles.css`, `./assets/…`) so it works on the `/toolbelt/` subpath
- Synthwave theme aligned with [`identidade-visual.md`](identidade-visual.md)
- Optional PT/EN toggle (pt-BR is the primary language)

## Publishing

The [`.github/workflows/pages.yml`](../.github/workflows/pages.yml) workflow publishes the `site/` directory via GitHub Actions:

- triggers on `push` to `main` that changes `site/**` or the workflow itself
- also via `workflow_dispatch`
- minimal permissions: `contents: read`, `pages: write`, `id-token: write`
- concurrency group `pages`
- actions pinned by version (`configure-pages`, `upload-pages-artifact`, `deploy-pages`)

### Enable Pages on the repository

Pages **still must be enabled manually** by a repository admin. The workflow does **not** use `enablement: true`.

1. Open **Settings → Pages**
2. Under **Build and deployment → Source**, choose **GitHub Actions**
3. Merge to `main` (or run the workflow manually) after the site exists in `site/`

Until that is configured, the workflow may fail on the deploy job because the `github-pages` environment is missing — that is expected.

## Local preview

```bash
cd site
python3 -m http.server 8080
# open http://127.0.0.1:8080/
```

Check internal links (`#problema`, `#comandos`, `#mcp`, `#instalar`, `#docs`), the Copy button on command blocks, and layout on a narrow viewport.
