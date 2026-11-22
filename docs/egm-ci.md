🇺🇸 English | [🇧🇷 Português](pt-BR/egm-ci.md)

# EGM gate in CI

The local `tb egm` result is self-reported: the process being judged can edit its state, its config and its agent. In CI, `tb stack egm --egm-mode ci` checks each gated document's committed **receipt** against the commit, then runs **one fresh review round** with CI's own config and credential. The design and its limits are in the [trust spec](specs/egm-trust.md). The command reference is in [`docs/cli.md`](cli.md).

## 1. Author workflow

For each spec or ADR (`egm.doc_globs`, default `docs/specs/**` and `docs/adr/**`):

```bash
tb egm review docs/specs/x.md            # then edit, resolve and loop until it converges
tb egm loop next docs/specs/x.md
git commit -am "docs: x"                   # the document and its sources first
tb egm receipt docs/specs/x.md            # .toolbelt/egm/receipts/<slug>.json
git add .toolbelt/egm/receipts && git commit -m "docs: egm receipt for x"
```

`tb egm receipt` refuses unless the verdict is `PASS` or `PASS_PARTIAL`, and refuses with `receipt_uncommitted` when the document or a source differs from `HEAD`. Running `tb stack egm` locally warns `receipt_missing` or `receipt_stale` before you push.

## 2. The workflow

Copy [`docs/examples/egm-ci.yml`](examples/egm-ci.yml) to `.github/workflows/egm.yml` and:

- set `TB_VERSION` to a release that has `--egm-mode ci`, and `TB_SHA256` to that archive's line in the release's `checksums.txt` ([docs/release.md](release.md));
- pin `CLAUDE_CODE_VERSION`;
- add the `ANTHROPIC_API_KEY` repository secret.

The workflow writes CI's egm config in `$RUNNER_TEMP` (never from the repository), passes the PR's base and head SHAs, and runs on every `pull_request` without a `paths` filter. A PR that touches no gated document passes with a `no_gated_docs` warning.

The example starts with `continue-on-error: true`. **While it is there the job succeeds even when the gate fails, so the check is advisory and making it required protects nothing.** Keep it until you have measured how often a fresh round finds a new `major` on documents that converged locally (spec, Open Questions); then drop it.

## 3. Repository settings

The check is only as strong as the workflow's protection. In ci mode the gate globs are the union of the merge-base's and head's, so a PR cannot drop a document from the gate by editing `.toolbelt/config.yaml`. Without these settings, the agent under review can edit the workflow in its own PR.

- **Required status check:** make the `egm` job required on the default branch.
- **Branch protection:** require pull requests and the status check, with **"include administrators"** (or the equivalent ruleset with no bypass).
- **CODEOWNERS**, with at least one owner who is not the author of the PRs under review:

  ```text
  /.github/CODEOWNERS        @owner-a @owner-b
  /.github/workflows/**      @owner-a @owner-b
  /.toolbelt/config.yaml     @owner-a @owner-b
  ```

  Enable "Require review from Code Owners". Do **not** put `.toolbelt/egm/receipts/**` under CODEOWNERS: authors update receipts on every change, and CI checks them anyway.

## 4. What fails, and what to do

| Result | Meaning | What to do |
|--------|---------|------------|
| `no_receipt`, `receipt_invalid`, `receipt_stale` | The receipt is missing, malformed, or older than the document or its sources | Converge, commit, `tb egm receipt`, commit |
| `ci_finding` | The fresh round found a `major` or `blocker` that the receipt does not account for | Fix it locally and refresh the receipt |
| `ci_downgrade` | The finding matches a claim the author declined or force-settled | A human decides: fix it, or merge through the repository's own override |
| `ci_upheld` | CI's arbiter upheld a finding the receipt says the arbiter withdrew | Fix it locally |
| `ci_partial`, `ci_not_run` | The fresh round was partial (a downgrade, a reviewer that did not complete) | Check the job log; re-run if it was transient. Every reviewer `not_run` usually means a wrong or revoked `ANTHROPIC_API_KEY`: check the secret |
| `ci_offline` | No credential (a fork PR gets no secrets, or the key is unset) | Run the job from a branch of the repository, or accept `--egm-allow-offline` and its `PASS_PARTIAL` ceiling |
| `ci_budget` | More documents than `egm.ci.max_docs`, or the step exceeded `egm.max_agent_runs` | Split the PR, or raise the limits in CI's config |
| `gated_doc_removed` | A gated document was deleted or moved out of the gate | A human reviews the removal |

## 5. Symbol citations and v3 receipts

A document may cite a Go symbol, `internal/x/x.go#Name` or `#Recv.Name` ([Phase C spec](specs/egm-phase-c.md)). Review verifies the symbol exists on the snapshot (a missing or ambiguous name is a `major` finding), and with `egm.receipt.write_v3: true` in your user config the receipt is version 3: it adds `symbols`, the sha256 of each cited symbol's raw bytes. Upgrade the CI's `tb` before anyone enables it: an older tb rejects a v3 receipt as `receipt_invalid`.

- **Freshness:** CI also recomputes each symbol hash at head, never trusting the receipt: a mismatch is `receipt_stale`, a symbol key the document does not cite is `receipt_invalid`.
- **Drift:** for a source cited only through symbols, `ci_source_drift` (and `tb egm drift-report`) reports `symbol_changed` or `symbol_missing` when a cited symbol's bytes change or vanish. An edit elsewhere in the same file is **not** drift: it is listed as `outside_symbols` and the document stays fresh. To get file-level drift back, also cite the file plainly (`internal/x/x.go`). A change to a helper that the cited symbol calls is outside the symbol, so cite the helper too if it matters.
- **Receipts of version 1 and 2** keep working and are compared by file, as before.

## Limits

- The fresh round is **one sample**: a `major` that both the local loop and one CI round miss gets merged.
- In a repository whose only code owner is also the admin whose `gh` token the agent can use, the agent can still change the protection or merge directly. tb cannot detect a missing CODEOWNERS file or branch protection.
- The example has not yet run on a hosted runner. It assumes Landlock (Ubuntu 24.04 kernels enable it) and that claude from npm runs with node's prefix as the only extra read-only path.
- Supply chain: the tb archive is checked by sha256 only (the cosign signature of `checksums.txt` is not verified), and npm installs claude without an integrity pin. Pin `actions/checkout` to a commit SHA if your policy requires it.
- Cost: one round per selected document (one agent run per required reviewer), plus at most one arbiter run per re-arbitrated claim.
