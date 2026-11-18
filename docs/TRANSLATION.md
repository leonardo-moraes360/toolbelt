🇺🇸 English | [🇧🇷 Português](pt-BR/TRANSLATION.md)

# Documentation translation

## Source of truth

**English is the source of truth** for project documentation. Translations live under `docs/<language_code>/`, mirroring the English path. Only English docs live at the repository root.

| English | Portuguese (pt-BR) |
|---------|--------------------|
| `README.md` | `docs/pt-BR/README.md` |
| `CONTRIBUTING.md` | `docs/pt-BR/CONTRIBUTING.md` |
| `CODE_OF_CONDUCT.md` | `docs/pt-BR/CODE_OF_CONDUCT.md` |
| `SECURITY.md` | `docs/pt-BR/SECURITY.md` |
| `CHANGELOG.md` | `docs/pt-BR/CHANGELOG.md` |
| `docs/**/*.md` (except `docs/<language_code>/`) | `docs/pt-BR/**/*.md` (same relative path) |

`AGENTS.md`, `CLAUDE.md` and the `.github/` templates are English only.

## Workflow

1. Edit the English file first.
2. Update every translation in the same change (same PR / commit series).
3. Keep the language selector on the first line of both files.
4. Point internal links in a translation at the same language when a counterpart exists (`docs/pt-BR/...`); otherwise link the English file.
5. Do **not** translate: CLI commands, flags, JSON field names, code, or literal CLI output as printed by the `tb` binary.

## Adding a language

Create `docs/<language_code>/` (BCP 47 tag, e.g. `es`, `pt-BR`) with the same relative paths and add the language to the selector line of each English file.

## CI

`TestDocsTranslationMirrorComplete` fails if a `docs/**/*.md` English file (outside `docs/<language_code>/`) or a root doc lacks its `docs/pt-BR/` twin.

`TestDocsCLIExamplesMatchCobra` validates `tb …` examples in `docs/cli.md` against the Cobra command tree.
