# llama.cpp advisory exceptions

ADR-018: an advisory that affects the pinned tag (its fixed-in tag is above
the pin, or it has no fix) blocks releases. The only exception is an
advisory in a component that is not vendored or not compiled here (server,
rpc, a GPU backend, ...). Such an exception lasts 30 days from its date and
is added in a reviewed PR, one row per advisory.

| Advisory | Component | Why it does not apply | Date | Expires |
|----------|-----------|-----------------------|------|---------|
| GHSA-j8rj-fmpv-wcxw | ggml RPC backend (`ggml/src/ggml-rpc`) | Not vendored and not compiled: the subset has no `ggml-rpc`, and `GGML_RPC` is forced off (`internal/embed/native/CMakeLists.txt`); tb never opens a socket for inference | 2026-11-21 | 2026-12-21 |
