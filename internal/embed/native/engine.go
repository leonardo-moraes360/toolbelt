// Package native is the only Go package that calls C (ADR-018). It wraps
// the tbembed shim over the vendored llama.cpp; builds without the
// embednative tag (or without cgo) carry no native code and report the
// engine as unavailable.
package native

import "errors"

// The vendored engine; TestManifestMatches keeps these equal to
// third_party/llama.cpp/MANIFEST.
const (
	EngineName   = "llama.cpp"
	EngineTag    = "b12600"
	EngineCommit = "29b52d398b294509746ec0a3f28948a3107c0213"
)

// Errors of Open and Embed, by shim status.
var (
	ErrUnavailable = errors.New("embed_unavailable: this tb was built without the native engine")
	ErrArgs        = errors.New("embed: invalid argument")
	ErrModelLoad   = errors.New("model_load: the model did not load")
	ErrUnsupported = errors.New("model_unsupported: the model gives no sentence vector (pooling NONE or RANK)")
	ErrTokenize    = errors.New("embed: tokenization failed")
	ErrEval        = errors.New("embed: evaluation failed")
	ErrNoMem       = errors.New("embed: out of memory")
)

// Engine describes the engine compiled into this binary, for tb version.
func Engine() string {
	if !Available {
		return "unavailable (built without embednative)"
	}
	return EngineName + " " + EngineTag + " (" + EngineCommit[:12] + ")"
}
