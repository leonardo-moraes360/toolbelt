package vecsearch

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
)

// DefaultIdle is how long the MCP server keeps an unused child (spec, MCP).
const DefaultIdle = 5 * time.Minute

// ErrPoolClosed is returned after Close (server shutdown).
var ErrPoolClosed = errors.New("embed: pool closed")

// poolKey is every StartOptions field that changes the child; a different
// key means the warm child no longer matches the config.
type poolKey struct {
	exe, args, model, sha       string
	threads                     int
	timeout                     time.Duration
	allowUnsandboxed            bool
	queryPrefix, docPrefix, rep string
}

func keyOf(o *embed.StartOptions) poolKey {
	return poolKey{
		exe: o.Exe, args: strings.Join(o.Args, "\x00"), model: o.Model, sha: o.SHA256,
		threads: o.Threads, timeout: o.Timeout, allowUnsandboxed: o.AllowUnsandboxed,
		queryPrefix: o.QueryPrefix, docPrefix: o.DocPrefix, rep: o.RepoRoot,
	}
}

// Pool keeps one warm child for the MCP server, so a query does not pay
// the model load each time. One lease is out at a time: the child serves
// one request stream, so concurrent tool calls queue on sem. The child is
// replaced when the key changes or it broke, and closed after idle
// without a lease.
type Pool struct {
	start StartFunc
	idle  time.Duration
	sem   chan struct{}

	mu          sync.Mutex
	emb         embed.Embedder
	key         poolKey
	unsandboxed bool
	broken      bool
	closed      bool
	timer       *time.Timer
	// gen invalidates idle timers that fired after the child was leased
	// or replaced.
	gen uint64
}

// NewPool returns a pool that starts children with start and closes them
// after idle unused.
func NewPool(start StartFunc, idle time.Duration) *Pool {
	return &Pool{start: start, idle: idle, sem: make(chan struct{}, 1)}
}

// Embedder implements Starter. It waits for the previous lease to end.
func (p *Pool) Embedder(ctx context.Context, o *embed.StartOptions) (emb embed.Embedder, release func(), unsandboxed bool, err error) {
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, false, ctx.Err()
	}
	child, unsandboxed, reused, err := p.acquire(ctx, o)
	if err != nil {
		<-p.sem
		return nil, nil, false, err
	}
	var once sync.Once
	return &lease{p: p, emb: child, opts: o, reused: reused}, func() { once.Do(p.release) }, unsandboxed, nil
}

// acquire runs with sem held, so no other lease can touch p.emb meanwhile;
// mu only guards against the idle timer and Close.
func (p *Pool) acquire(ctx context.Context, o *embed.StartOptions) (emb embed.Embedder, unsandboxed, reused bool, err error) {
	k := keyOf(o)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, false, false, ErrPoolClosed
	}
	p.stopTimerLocked()
	var old embed.Embedder
	if p.emb != nil && (p.broken || p.key != k) {
		old, p.emb = p.emb, nil
	}
	cur, curUnsandboxed := p.emb, p.unsandboxed
	p.mu.Unlock()
	closeEmbedder(old)
	if cur != nil {
		return cur, curUnsandboxed, true, nil
	}
	emb, unsandboxed, err = p.startLocked(ctx, o, &k)
	return emb, unsandboxed, false, err
}

// startLocked starts a child and installs it; sem must be held.
func (p *Pool) startLocked(ctx context.Context, o *embed.StartOptions, k *poolKey) (embed.Embedder, bool, error) {
	emb, unsandboxed, err := p.start(ctx, o)
	if err != nil {
		return nil, false, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		closeEmbedder(emb)
		return nil, false, ErrPoolClosed
	}
	p.emb, p.key, p.unsandboxed, p.broken = emb, *k, unsandboxed, false
	return emb, unsandboxed, nil
}

// restart replaces a warm child that died while idle; sem must be held.
func (p *Pool) restart(ctx context.Context, o *embed.StartOptions) (embed.Embedder, error) {
	p.mu.Lock()
	old := p.emb
	p.emb = nil
	p.mu.Unlock()
	closeEmbedder(old)
	k := keyOf(o)
	emb, _, err := p.startLocked(ctx, o, &k)
	return emb, err
}

func (p *Pool) release() {
	p.mu.Lock()
	var old embed.Embedder
	switch {
	case p.emb == nil:
	case p.broken || p.closed:
		old, p.emb, p.broken = p.emb, nil, false
	default:
		p.gen++
		gen := p.gen
		p.timer = time.AfterFunc(p.idle, func() { p.expire(gen) })
	}
	p.mu.Unlock()
	closeEmbedder(old)
	<-p.sem
}

func (p *Pool) expire(gen uint64) {
	p.mu.Lock()
	if gen != p.gen || p.emb == nil {
		p.mu.Unlock()
		return
	}
	old := p.emb
	p.emb = nil
	p.mu.Unlock()
	closeEmbedder(old)
}

func (p *Pool) stopTimerLocked() {
	p.gen++
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
}

func (p *Pool) markBroken() {
	p.mu.Lock()
	p.broken = true
	p.mu.Unlock()
}

// Close kills the warm child and refuses later leases. It waits for a
// lease in progress, so the child is never closed under a request.
func (p *Pool) Close() error {
	p.mu.Lock()
	p.closed = true
	p.stopTimerLocked()
	p.mu.Unlock()
	p.sem <- struct{}{}
	p.mu.Lock()
	old := p.emb
	p.emb = nil
	p.mu.Unlock()
	closeEmbedder(old)
	<-p.sem
	return nil
}

func closeEmbedder(e embed.Embedder) {
	if e != nil {
		_ = e.Close()
	}
}

// lease is the pooled child as one caller sees it: Close is a no-op (the
// pool owns the child) and a fatal Embed error marks it for replacement.
type lease struct {
	p    *Pool
	emb  embed.Embedder
	opts *embed.StartOptions
	// reused marks a child started by an earlier lease: it may have died
	// while idle (OOM, killed), which only shows on this lease's first call.
	reused, retried bool
}

func (l *lease) Info() embed.Info { return l.emb.Info() }

func (l *lease) Embed(ctx context.Context, kind embed.Kind, texts []string) (vecs [][]float32, truncated []bool, err error) {
	vecs, truncated, err = l.emb.Embed(ctx, kind, texts)
	if err != nil && l.reused && !l.retried && errors.Is(err, embed.ErrEmbedCrashed) {
		// One retry on a fresh child, so an idle death costs a restart, not
		// a failed query. A crash on the fresh child is reported.
		l.retried = true
		if fresh, serr := l.p.restart(ctx, l.opts); serr == nil {
			l.emb = fresh
			vecs, truncated, err = l.emb.Embed(ctx, kind, texts)
		}
	}
	if err != nil && brokenChild(err) {
		l.p.markBroken()
	}
	return vecs, truncated, err
}

func (l *lease) Close() error { return nil }
