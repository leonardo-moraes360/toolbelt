package embed

import (
	"context"
	"errors"
	"fmt"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

// Kind selects the task prefix the parent adds (spec, Protocol).
type Kind string

const (
	KindDoc   Kind = "doc"
	KindQuery Kind = "query"
)

// Info is the child's handshake: the store header records it, so a store
// built with another model or engine is stale.
type Info struct {
	Engine      string `json:"engine"`
	Commit      string `json:"commit"`
	ModelSHA256 string `json:"model_sha256"`
	Dims        int    `json:"dims"`
	NCtx        int    `json:"n_ctx"`
}

// Embedder is what tb graph needs from the child; tests use in-memory fakes.
type Embedder interface {
	Info() Info
	// Embed returns one L2-normalized vector per text, in order, and whether
	// each text was cut to the model context.
	Embed(ctx context.Context, kind Kind, texts []string) (vecs [][]float32, truncated []bool, err error)
	Close() error
}

// Child failures (spec reason codes). None of them may leave graph.vec
// half-written: the caller writes the store only after every batch succeeds.
var (
	ErrEmbedProtocol    = errors.New("embed_protocol")
	ErrEmbedCrashed     = errors.New("embed_crashed")
	ErrEmbedTimeout     = errors.New("embed_timeout")
	ErrEmbedUnsandboxed = errors.New("embed_unsandboxed")
	// ErrEmbedRequest is a per-request error the child reported (the child
	// keeps serving).
	ErrEmbedRequest     = errors.New("embed_request")
	ErrEmbedUnavailable = native.ErrUnavailable
)

// ChildError is an error reply from the child. Msg is untrusted: Error()
// quotes it (no terminal escapes) and caps it, under ErrEmbedRequest so it
// cannot pose as another reason code.
type ChildError struct{ Msg string }

const maxChildError = 512

func (e *ChildError) Error() string {
	msg := e.Msg
	if len(msg) > maxChildError {
		msg = msg[:maxChildError] + "…"
	}
	return fmt.Sprintf("%s: child replied %q", ErrEmbedRequest, msg)
}

func (e *ChildError) Unwrap() error { return ErrEmbedRequest }

// Protocol limits shared by child and parent.
const (
	MaxTexts        = 64
	MaxRequestBytes = 1 << 20
)
