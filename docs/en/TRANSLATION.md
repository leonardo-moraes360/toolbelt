[🇧🇷 Português](../TRANSLATION.md) | 🇺🇸 English

# Documentation translation

## Source of truth

**Brazilian Portuguese (pt-BR) is the source of truth** for project documentation.

| Portuguese | English mirror |
|------------|----------------|
| `README.md` | `README.en.md` |
| `CONTRIBUTING.md` | `CONTRIBUTING.en.md` |
| `CODE_OF_CONDUCT.md` | `CODE_OF_CONDUCT.en.md` |
| `SECURITY.md` | `SECURITY.en.md` |
| `docs/**/*.md` (except `docs/en/`) | `docs/en/**/*.md` (same relative path) |
| `CHANGELOG.md` | Portuguese only (Keep a Changelog / semver links may stay pt-BR) |

`docs/en/TRANSLATION.md` is English-only guidance (this file). An optional short PT pointer may live at `docs/TRANSLATION.md`.

## Workflow

1. Edit the Portuguese file first.
2. Update the English mirror in the same change (same PR / commit series).
3. Keep language selectors at the top of both files.
4. Point internal links in EN files at EN counterparts (`docs/en/...`, `README.en.md`, relative `../cli.md` inside `docs/en/`, etc.).
5. Do **not** translate: CLI commands, flags, JSON field names, code, or literal CLI output as printed by the `tb` binary (many messages are pt-BR on purpose).

## CI

`TestDocsEnglishMirrorComplete` fails if any `docs/**/*.md` outside `docs/en/` lacks a same-path twin under `docs/en/`.

`TestDocsCLIExamplesMatchCobra` also validates `tb …` examples in `docs/en/cli.md` against the Cobra command tree.
