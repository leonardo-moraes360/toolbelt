[🇺🇸 English](../../../campo/rodada2/PROMPT.md) | 🇧🇷 Português

Field study ROUND 2 for toolbelt (`tb`), the Go CLI + MCP server in THIS repo. Round 1 (docs/campo/{SLUG}.md and docs/campo/SINTESE.md) found 12 problems, and all 12 fixes are now merged in main. Goal of round 2: re-run the same kind of real usage on ONE public third-party repository, {REPO} (https://github.com/{REPO}), group: {GROUP}, to (a) confirm or refute that each of the 12 fixes works in practice, and (b) find what is still wrong or new.

Rules:
- NEVER push, open issues, open PRs, comment or interact in any way with {REPO} or its upstream. Work only on a local clone inside your VM (e.g. /tmp/target). No secrets, no network calls beyond cloning and installing dependencies.
- In THIS toolbelt repo, change ONLY the new file `docs/campo/rodada2/{SLUG}.md`, on a new branch `campo2/{SLUG}` created from current main. Commit with author `marin-grok-bot <aak16j9t+marin@mail.grokbot.com>` (e.g. `git -c user.name=marin-grok-bot -c user.email=aak16j9t+marin@mail.grokbot.com commit ...`), push that branch, and do NOT open a pull request. Do not modify any tb source code.

Steps:
1. Build tb from current main. Read README.md, docs/cli.md, CHANGELOG.md, docs/campo/SINTESE.md and the round 1 report docs/campo/{SLUG}.md.
2. Clone {REPO} (shallow is fine) and set up its build/test/lint toolchain as far as feasible. Note what you could not set up.
3. Pick ONE realistic small-to-medium change (bug fix or small feature with tests). Prefer a different task from round 1 so results are not memorised; say which one round 1 used.
4. Implement it locally using tb end to end: `tb init` (including toolchain detection and AGENTS.md/CLAUDE.md handling, try `--write-agents` if no instrumentation exists), `tb mem`, `tb graph` (build/impact/query; for Python repos check `calls` edges), tickets with a `## Scope`/`## Escopo` section using Markdown lists and `**` globs, `tb verify` (with build_cmd, lint_cmd, test_cmd and hidden_test_cmd as applicable), `tb review`, `tb drift`, `tb guard` (including guard.yaml protected_paths and `--strict --json` exit code), `tb bench` (default_agent and `compare --agent`), and the MCP server if you can wire it. Also run the repo's own native checks to compare what tb caught vs missed.
5. Record precise evidence per command: exact command, what happened, time, helped/hindered, false positives/negatives, crashes, confusing UX/docs.

Write `docs/campo/rodada2/{SLUG}.md` (pt-BR) with exactly these sections:
## Repositório  (name, language, size, AI instrumentation files, toolchain set up or not)
## Tarefa escolhida  (what and why; issue link if any; completed or not; native tests/lint/build result; round 1 task for comparison)
## Verificação das 12 correções  (table: correção | funcionou/não funcionou/não testado/não se aplica | evidência). The 12: 1 graph SkipDirs at the walk root; 2 verify hidden_test_cmd without tests/hidden/; 3 guard --strict --json non-zero exit; 4 guard reads protected_paths from guard.yaml; 5 init toolchain detection; 6 review tests/hidden matched by path; 7 bench honors default_agent and compare --agent; 8 review parses ## Scope/## Escopo, Markdown lists and ** globs; 9 verify build_cmd and lint_cmd; 10 init reads AGENTS.md/CLAUDE.md and --write-agents; 11 graph Python calls edges; 12 verify visual skipped aggregation and CLI docs aligned.
## Uso do tb por comando  (table: comando | resultado | ajudou/atrapalhou/neutro | evidência)
## Problemas que continuam  (round 1 problems from this repo's report that still happen)
## Problemas novos  (each: severity P0-P3, reproduction steps, expected vs actual; include regressions caused by the fixes)
## O que o tb pegou e o que deixou passar
## Interação com instrumentação existente / onboarding
## Comparação com a rodada 1  (better/worse/same, concretely)
## Sugestões de melhoria  (concrete, ranked, tagged by tb command/area)
## Métricas  (wall time, tb invocations, tb failures; compare with round 1 numbers)
Be factual; do not invent results. If something could not be run, say so and why.
