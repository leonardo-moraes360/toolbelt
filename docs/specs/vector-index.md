🇺🇸 English | [🇧🇷 Português](../pt-BR/specs/vector-index.md)

# Optional vector index for `tb graph search` — spec

Status: **V0, V1 and V2 implemented**; no longer experimental: recall@10 on 44 intent queries over this repository (all-MiniLM-L6-v2 Q8_0): lexical 0.273, semantic 0.670, hybrid 0.568, so hybrid − lexical = +0.295 ≥ +0.10 ([dogfood](../campo/dogfood.md) #45). Semantic mode scored highest on this suite; hybrid keeps exact-name matches. The shim packs a request's texts into shared evaluations (`n_seq_max` 64, unified KV, a 512-token budget); on the x86-64-v2 floor that saves about 4% when indexing and about 21% on short texts ([dogfood](../campo/dogfood.md) #47). Decision record: [ADR-018](../adr/018-cgo-componentes-nativos.md) (cgo, vendored llama.cpp, C/C++ quality gate).

The vector index adds semantic search to the code graph:

- **Embeddings** come from a GGUF model, run by vendored llama.cpp in a sandboxed child process without network access.
- **Storage** is a file next to `graph.json`.
- **Search** fuses the BM25 ranking with vector similarity.

The feature is opt-in. Off, it changes no current output.

It ships in three phases, each usable alone:

- **V0:** cgo release and the native gate. Every target is built with cgo, with vendored llama.cpp, the C shim and the `native` CI job. The only visible change is the engine line in `tb version`.
- **V1:** embeddings and CLI search.
  - the `tb __graph-embed` child and the sandbox network block;
  - the shared `internal/secrets` package;
  - the vector store, `tb graph index --vectors` and `tb graph search --mode`.
- **V2:** MCP and evidence.
  - the `mode` and `vectors` parameters of the MCP tools;
  - a warm child in `tb mcp`;
  - the evaluation that decides whether the feature leaves experimental status.

## Problem

- **Lexical search misses intent.** `tb graph search` ranks titles, paths and packages by BM25. A query such as "where do we refuse symlinked sources" finds nothing unless it shares words with a name (`lstatChain`), and an agent then falls back to grep.
- **No local semantic option.** ADR-003 and ADR-006 left embeddings to "Phase 2", and the only design on record assumed sqlite-vec. Neither fits a `graph.json` index or the project's offline, verified-release posture.
- **Unproven value.** The field study ([SINTESE.md](../campo/SINTESE.md)) did not observe this failure. The feature must measure its gain before the roadmap item is closed.

## Goals

- G1: semantic and hybrid search over the code and docs that the graph indexes, offline, with no network and no external service.
- G2: no change for users who do not opt in. `tb graph search` without `--mode`, its JSON and the MCP tool defaults stay byte-identical.
- G3: native code reaches neither the repository, nor HOME, nor the network. Inference runs in a sandboxed child, and the model is pinned by hash.
- G4: an evaluation on this repository that reports recall@10 for the lexical and hybrid modes.

## Non-Goals

- GPU backends, CPU builds above the ADR-018 floor, model download by tb, remote embedding APIs.
- Node-level (function-level) vectors: graph nodes carry no line ranges today, so V1 embeds file chunks (see Units).
- Using vectors in `callers`, `impact`, `explain` or `path`, or as EGM sources.
- Replacing BM25: lexical stays the default mode.

## Configuration

`tb graph` reads its config through `config.Load()`, which merges `.toolbelt/config.yaml`. The new keys must not come from the repository, because they choose what native code loads. V1 adds a user-only read:

- **`config.LoadUser`:** egm's `LoadUserConfig` generalized to any section. It reads the `-c` file, or the global config, and never the repository's file.
- **Repository filter:** `graph` loads `graph.vector` and `graph.embed` only through `LoadUser`.
- **Warning:** a repository `.toolbelt/config.yaml` that sets any of them gets a `config_untrusted` warning (the same code as egm, R34), and the value is ignored.
- **MCP:** `tb mcp` uses the same path.

| Key | Default | Meaning |
|-----|---------|---------|
| `graph.vector.enabled` | `false` | Turns the feature on; without it `--vectors` and `--mode semantic|hybrid` fail with `vectors_disabled` |
| `graph.embed.model` | — | Absolute path to a GGUF embedding model |
| `graph.embed.model_sha256` | — | Required; a mismatch refuses to load (`model_mismatch`) |
| `graph.embed.query_prefix`, `doc_prefix` | `""` | Task prefixes some models need (e.g. `search_query: `) |
| `graph.embed.threads` | `min(4, NumCPU)` | Inference threads |
| `graph.embed.timeout` | `2m` | Per batch; a timeout kills the child (`embed_timeout`) |
| `graph.embed.allow_unsandboxed` | `false` | Without the full sandbox (no Landlock or seccomp on Linux; darwin; windows) the child refuses with `embed_unsandboxed` unless this is true |
| `graph.vector.max_chunks` | `50000` | Above it, indexing refuses (`vectors_too_many`) |
| `graph.vector.chunk_lines`, `chunk_bytes`, `overlap_lines` | `40`, `1536`, `8` | Chunk shape: at most `chunk_lines` lines and `chunk_bytes` bytes |

The model file is checked by the parent, and again by the child before loading:
- the path is absolute;
- every component is reached through `Lstat` with no symlink (on macOS, `/tmp` and `/var` are symlinks: use the `/private/...` path);
- every parent directory belongs to the user or root and no other user can write it (sticky directories such as `/tmp` excepted; group write only for the user's own primary group) (unix);
- the file is opened without following a symlink, is regular and owned by the current user;
- it is outside the repository (V1: the repository is known only to `tb graph`);
- its sha256 equals `model_sha256`;
- after the native load, the path still names the hashed file (same device, inode, size and mtime), or the load is refused (`model_mismatch`).

## Units

A **unit** is a chunk of a file that the graph indexer already reads (its languages and docs, within `graph.max_file_bytes`).

- **Shared secrets package:** `internal/egm` imports `internal/graph`, so graph cannot import egm. V1 moves `Redact` and the R7 deny globs (`denyBaseGlob`, `denyPathGlob`) into a leaf package, `internal/secrets`, next to `mem.SecretPatterns`. egm and graph both import it, and egm keeps its behavior (its tests must not change).
- **Skipped files:** a file whose name matches the deny list is never read for embedding.
- **Redaction first:** the **whole file** is redacted before chunking, so a secret that spans a chunk boundary (a PEM block) is still caught. A secret reaches the child only as `[REDACTED]`.
- **Chunking:** chunks are cut on line boundaries, at most `chunk_lines` lines and `chunk_bytes` bytes, with `overlap_lines` of overlap. A single line over `chunk_bytes` is cut at a UTF-8 boundary.
- **Identity:** each unit records its path, line range, the sha256 of its redacted text, and a `truncated` flag. The flag is set when the child had to cut the unit to the model context.

A semantic hit is reported as the file node, with the best chunk's line range as `lines`. Docs nodes (vault and Markdown) map the same way.

## The child: `tb __graph-embed`

The parent starts the child by re-executing its own binary (`os.Executable()`) as `tb __graph-embed`, with the model path and expected hash as arguments.

### Sandbox

The sandbox gains `DenyNetwork`, a seccomp filter installed by the trampoline: `socket(2)` and `socketpair(2)` fail with `EPERM`. Where Landlock ABI ≥ 6 is available, it also scopes abstract unix sockets and signals.

The child runs with:
- `DenyNetwork`;
- read access only to the directories the sandbox always grants (`/usr`, `/lib*`, `/etc`, `/proc`, `/sys`, …), plus the model file (read) and the `tb` binary (read and execute);
- an **empty private temp dir** as a read-only workdir;
- a private tmp, an empty HOME, `MinimalDev` and the minimal environment.

It can therefore read neither the repository nor HOME, and it has no network.

Fallbacks:
- **Linux without Landlock or seccomp, darwin and windows:** the child refuses (`embed_unsandboxed`) unless `allow_unsandboxed` is set. Then it runs as a separate process with the minimal environment, and every command that used it prints an `embed_unsandboxed` warning.
- **Build without the `embednative` tag:** the child answers `embed_unavailable`.

### Engine settings in the shim

- A model whose pooling type is `NONE` is refused (`model_unsupported`): it gives no sentence vector.
- `n_seq_max = 64` with a unified KV buffer, so each sequence keeps the full `n_ctx = min(n_ctx_train, 8192)`. A request is packed in order into evaluations of at most 512 tokens, and a single longer text runs alone (up to `n_ctx`): a non-causal model reads one evaluation in a single ubatch and its attention grows with the square of the tokens, so a larger budget made long chunks slower (budget 2048: +6% on 64 chunks of 1.5 KB). Sequences do not attend to each other: batched and single-call vectors agree to cosine ≥ 0.999 (`TestEmbedBatchMatchesSingle`).
- A text longer than `n_ctx` tokens keeps its first `n_ctx − 1` tokens and its last one (the trailing special token).
- Vectors are read per sequence and L2-normalized.

### Protocol

JSON lines on stdin and stdout:

1. **Handshake:** after verifying the hash and loading, the child writes `{"v":1,"engine":"llama.cpp","commit":"…","model_sha256":"…","dims":N,"n_ctx":M}`.
2. **Requests:** each request is `{"id":k,"kind":"doc"|"query","texts":["…"]}`, with at most 64 texts and 1 MiB.
3. **Replies:** each reply is `{"id":k,"vectors":["<base64 of dims little-endian float32>",…],"truncated":[i,…]}` or `{"id":k,"error":"…"}`. `truncated` lists the indexes of the texts that were cut to `n_ctx` tokens.

Limits, all enforced by the parent:
- **Reply size:** each reply line is capped at `64 × ceil(dims × 4 × 4/3) + 64 KiB`. There is no lifetime cap, because the warm child serves many requests.
- **Protocol errors:** a longer or malformed line, a wrong `id`, or a vector of the wrong length kills the child and fails with `embed_protocol`.
- **Crashes:** a crash or signal fails with `embed_crashed` and the exit status.

The parent adds the prefixes (`query_prefix` / `doc_prefix`).

## Vector store

The store is `.toolbelt/graph.vec`, next to `graph.json`, with mode 0600, written atomically (temp file plus rename).

- **Layout:**
  - the magic `TBVEC1`;
  - a length-prefixed JSON header: `v`, `engine_commit`, `model_sha256`, `dims`, the chunk shape, `prefixes_sha256`, `graph_fingerprint` (the `Fingerprint` of the `graph.json` index it was built against) and `count`;
  - `count` records of path, line range, text sha256, `truncated`, and `dims` little-endian float32.
- **Reading:** the header must parse, `count` must be at most `max_chunks`, and the file size must match exactly. Otherwise the store is `vectors_invalid`, and tb rebuilds it rather than reading past the end.
- **Freshness:** the store is **stale** when any of these differ from the current config: `model_sha256`, `dims`, the chunk shape or the prefixes. A stale store is never queried (`vectors_stale`).
- **Incremental:** `--vectors` re-embeds only the units whose text sha256 is not already stored with the same model, and drops the units of files that are gone.

## CLI

```bash
tb graph index --vectors                 # index, then build or update graph.vec
tb graph search "<query>" --mode hybrid  # lexical (default) | semantic | hybrid
```

- **Modes:**
  - **lexical:** today's ranking, unchanged.
  - **semantic:** cosine similarity of the query vector against every unit (brute force), grouped by file node (best chunk), top `--limit`. A hit whose file node is no longer in the index is dropped.
  - **hybrid:** Reciprocal Rank Fusion with `k = 60` over the lexical ranking and the semantic ranking. Ties are broken by node ID.
- **JSON:** in `semantic` and `hybrid` each hit gains `lines`, `score_semantic` and, in hybrid, `rank_lexical`, `rank_semantic` and `score_fused`. The lexical JSON is unchanged.
- **Stale vectors:** when `graph_fingerprint` differs from the current index, `semantic` and `hybrid` still answer, with `"stale_vectors": true` and a warning. `--mode` never reindexes.

## MCP (V2)

- `tb_graph_search` gains `mode` (`lexical` by default).
- `tb_graph_index` gains `vectors` (boolean, `false` by default).
- `tb mcp` keeps one warm child per server, killed after 5 minutes idle and on exit, so a query does not pay the model load each time.
- Errors map to the same reasons.

## Evaluation (V2)

`internal/graph/eval/semantic.yaml` holds at least 25 queries over this repository. Each query lists the expected paths, written as intent ("where are symlinked sources refused"), not as names.

A test gated on `TB_EMBED_MODEL` (skipped without it) reports recall@10 for the lexical, semantic and hybrid modes. The result, with the model and engine commit, is recorded in [dogfood.md](../campo/dogfood.md).

The feature leaves experimental status (README roadmap item closed) only if the hybrid mode's recall@10 is at least the lexical one plus 0.10. Otherwise the docs keep marking it experimental and the item stays open.

## CI smoke model

V0 pins one small GGUF embedding model for CI:
- at most 50 MB;
- a permissive license;
- a pooling type other than `NONE`.

The candidate is all-MiniLM-L6-v2 quantized to Q8_0 (Apache-2.0). Its URL and sha256 go in `.github/embed-smoke-model.txt`. CI downloads it with the hash checked and caches it; it is never committed. On darwin and windows, the smoke test runs with a test config that sets `allow_unsandboxed: true`.

## Security summary

| Threat | Mitigation |
|--------|------------|
| Hostile model file (GGUF parser bug) | Path and sha256 from the user config only; the file is user-owned and outside the repository; the hash is re-checked in the child; it is parsed in the sandboxed, network-less child |
| Memory bug in llama.cpp or the shim | Native gate (ADR-018): sanitizer build, libFuzzer, static analysis; out-of-process child at run time |
| Static initializers in every command | The CPU floor avoids SIGILL; the strace check shows that static init does no I/O |
| Secrets in embedded text | Deny list by name; whole-file `Redact` before chunking; `graph.vec` is 0600 and local (`.toolbelt/` is ignored) |
| A repository choosing what loads | Every new key is read through `config.LoadUser`; repository values are warned about and ignored |
| Resource exhaustion | `max_chunks`, the request size, the reply cap and the per-batch timeout |

## Phases

### V0: cgo release and native gate

These files change together:
- `third_party/llama.cpp/` (the allowlisted subset), its `MANIFEST` and `ADVISORIES.md`, and `scripts/vendor-llama.sh`;
- `internal/embed/native/` (the shim and its cgo bindings, build tag `embednative`);
- `.goreleaser.yaml` (`CGO_ENABLED=1`, `-tags embednative`, zig `CC`/`CXX`, per-build `hooks.pre` with CMake, per-target `CGO_LDFLAGS`, hardening flags);
- `.github/workflows/ci.yml` (the `native` job; the `import "C"` rule is a test, `TestOnlyNativeImportsC`), `release.yml` (the smoke matrix, QEMU, `rockylinux:8`), `codeql.yml` (`c-cpp` with the SARIF check step; the `go` job stays untagged);
- `.github/embed-smoke-model.txt`.

`tb version` prints `embed: llama.cpp <tag> (<commit>)`.

### V1: embeddings and CLI search

These files change together:
- `internal/secrets/` (moved from egm);
- `internal/sandbox` (`DenyNetwork`);
- `internal/embed/` (the child, its protocol and the parent client);
- `internal/graph/vector.go` (units, store, search, RRF);
- `cmd/tb/graph.go` (`--vectors`, `--mode`);
- `internal/config` (`LoadUser` and the keys).

### V2: MCP and evaluation

These files change together:
- `internal/mcp` (the parameters and the warm child);
- `internal/graph/eval/`.

The evaluation report goes into `docs/campo/dogfood.md`.

## Docs to update with the implementation

- **README:** the roadmap item and the `tb graph` row.
- **`docs/cli.md`:** `graph index` and `graph search`.
- **`docs/release.md`:** native artifacts, the glibc and CPU floors, the smoke matrix.
- **ADRs:**
  - ADR-018: status accepted after V0.
  - ADR-001, ADR-003 and ADR-014: the status notes already point here.
- **CHANGELOG**, and the pt-BR mirror of each.

## Acceptance criteria

### V0

1. Each of the six targets builds with cgo, through zig or the named fallback for that target. The spike result (the toolchain per target and the binary size against v0.0.10) is recorded in the V0 PR before ADR-018 is accepted.
2. The native smoke matrix runs `tb version` and an embed smoke test on all six targets, each on its own runner, plus x86-64-v2 under QEMU. A failure blocks the release.
3. Linux artifacts meet all of these:
   - they run in `rockylinux:8`;
   - `objdump -T` shows no symbol version above `GLIBC_2.28`;
   - `checksec`/`readelf` show PIE, full RELRO, NX, canaries and fortified calls.
4. `tb version` names the engine, tag and commit. The SBOM lists llama.cpp and ggml with the manifest's commit and subset hash.
5. The `native` job fails on each of these:
   - a shim compiler warning;
   - a `clang-tidy` or `cppcheck` finding;
   - a format diff;
   - a CodeQL alert under `internal/embed/native/`;
   - a vendored tree that does not match `MANIFEST`;
   - an ASan or UBSan report;
   - a libFuzzer crash;
   - an `import "C"` outside `internal/embed/native`;
   - a `tb version` strace that differs from the untagged build.
6. The advisory job fails on an advisory that affects the pinned tag (fixed-in above the pin, or no fix). An exception in `ADVISORIES.md` for a component that is not compiled holds for 30 days.
7. An untagged `go build ./...` and `go test ./...` still work, and `tb __graph-embed` answers `embed_unavailable`.

### V1

8. With `graph.vector.enabled` off, `tb graph search` output and JSON are byte-identical to the release before V1, and `--vectors`/`--mode semantic` fail with `vectors_disabled`.
9. Each of these does not load:
   - a model whose sha256 differs from `model_sha256` (`model_mismatch`);
   - a symlinked or repository-local model path (`model_path`);
   - a pooling-`NONE` model (`model_unsupported`).

   A repository config that sets a `graph.vector`/`graph.embed` key is ignored with `config_untrusted`.
10. A file on the deny list is never read for embedding. A secret, including one that spans a chunk boundary, reaches the child only as `[REDACTED]` (checked through a fake child that records its input). egm's redaction tests pass unchanged after the move to `internal/secrets`.
11. A second `--vectors` run with no file changes sends no text to the child. Editing one file re-embeds only that file's changed chunks.
12. Changing the model hash, the prefixes or the chunk shape makes the store stale: it is never queried (`vectors_stale`), and the next `--vectors` rebuilds it.
13. These child failures leave `graph.vec` unchanged:
    - a malformed or oversized reply line, or a vector of the wrong length (`embed_protocol`);
    - a crash (`embed_crashed`);
    - a timeout (`embed_timeout`).
14. On Linux with Landlock and seccomp, a hostile fake child cannot do any of these:
    - read a repository file or a HOME file;
    - write outside its private tmp;
    - create a socket of any family.

    Without them, and on darwin and windows, it refuses with `embed_unsandboxed` unless `allow_unsandboxed` is set.
15. `hybrid` fuses by RRF (`k = 60`) and is deterministic for equal inputs. A hit whose node is gone is dropped. A truncated or corrupt `graph.vec` is `vectors_invalid`, never a crash.

### V2

16. `tb_graph_search` without `mode` and `tb_graph_index` without `vectors` behave as before. With them, they match the CLI, and the warm child is reused across queries and killed after 5 minutes idle.
17. The evaluation test reports recall@10 for the three modes, and its result is recorded with the model and engine commit. The experimental mark is removed only when hybrid ≥ lexical + 0.10.
