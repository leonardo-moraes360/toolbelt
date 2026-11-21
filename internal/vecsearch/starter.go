package vecsearch

import (
	"context"
	"errors"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
)

// Starter hands out an embedder for one command or tool call. release must
// be called when the caller is done; the embedder's own Close is not.
// unsandboxed reports a child running without the full sandbox, so the
// caller can emit embed_unsandboxed.
type Starter interface {
	Embedder(ctx context.Context, o *embed.StartOptions) (emb embed.Embedder, release func(), unsandboxed bool, err error)
}

// StartFunc spawns a new embedder; tests replace StartClient with fakes.
type StartFunc func(ctx context.Context, o *embed.StartOptions) (emb embed.Embedder, unsandboxed bool, err error)

// StartClient starts the real sandboxed child.
func StartClient(ctx context.Context, o *embed.StartOptions) (embed.Embedder, bool, error) {
	c, err := embed.Start(ctx, o)
	if err != nil {
		return nil, false, err
	}
	unsandboxed := c.Unsandboxed()
	return c, unsandboxed, nil
}

// OneShot starts a child per call and closes it on release (the CLI, where
// the process ends with the command anyway).
type OneShot struct {
	// Start defaults to StartClient.
	Start StartFunc
}

// Embedder implements Starter.
func (s OneShot) Embedder(ctx context.Context, o *embed.StartOptions) (emb embed.Embedder, release func(), unsandboxed bool, err error) {
	start := s.Start
	if start == nil {
		start = StartClient
	}
	emb, unsandboxed, err = start(ctx, o)
	if err != nil {
		return nil, nil, false, err
	}
	return emb, func() { _ = emb.Close() }, unsandboxed, nil
}

// brokenChild reports an Embed error after which the child is dead: the
// client kills it on protocol faults, crashes, timeouts and cancellation,
// and every later call would fail the same way.
func brokenChild(err error) bool {
	for _, target := range []error{embed.ErrEmbedProtocol, embed.ErrEmbedCrashed, embed.ErrEmbedTimeout, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
