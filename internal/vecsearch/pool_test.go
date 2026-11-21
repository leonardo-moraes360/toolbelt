package vecsearch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
)

// fakeEmbedder is an in-memory child; failNext makes the next Embed return
// that error.
type fakeEmbedder struct {
	id       int
	closed   atomic.Bool
	mu       sync.Mutex
	failNext error
	busy     atomic.Int32
	overlap  atomic.Bool
}

func (f *fakeEmbedder) Info() embed.Info {
	return embed.Info{Engine: "fake", Commit: "c", ModelSHA256: "sha", Dims: 2, NCtx: 512}
}

func (f *fakeEmbedder) Embed(_ context.Context, _ embed.Kind, texts []string) (vecs [][]float32, truncated []bool, err error) {
	if f.busy.Add(1) > 1 {
		f.overlap.Store(true)
	}
	defer f.busy.Add(-1)
	time.Sleep(time.Millisecond)
	f.mu.Lock()
	err, f.failNext = f.failNext, nil
	f.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	if f.closed.Load() {
		return nil, nil, errors.New("embed: client closed")
	}
	for range texts {
		vecs = append(vecs, []float32{1, 0})
		truncated = append(truncated, false)
	}
	return vecs, truncated, nil
}

func (f *fakeEmbedder) fail(err error) {
	f.mu.Lock()
	f.failNext = err
	f.mu.Unlock()
}

func (f *fakeEmbedder) Close() error { f.closed.Store(true); return nil }

type fakeStarter struct {
	mu       sync.Mutex
	children []*fakeEmbedder
}

func (s *fakeStarter) start(_ context.Context, _ *embed.StartOptions) (embed.Embedder, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := &fakeEmbedder{id: len(s.children)}
	s.children = append(s.children, f)
	return f, false, nil
}

func (s *fakeStarter) starts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.children)
}

func (s *fakeStarter) child(i int) *fakeEmbedder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.children[i]
}

func opts() *embed.StartOptions {
	return &embed.StartOptions{Model: "/m.gguf", SHA256: "sha", Threads: 2, RepoRoot: "/repo"}
}

// query leases the pooled child for one embed, like a tool call.
func query(t *testing.T, p *Pool, o *embed.StartOptions) error {
	t.Helper()
	emb, release, _, err := p.Embedder(context.Background(), o)
	if err != nil {
		t.Fatalf("Embedder: %v", err)
	}
	defer release()
	_, _, err = emb.Embed(context.Background(), embed.KindQuery, []string{"q"})
	return err
}

func TestPoolReusesWarmChild(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	defer p.Close()
	for i := range 3 {
		if err := query(t, p, opts()); err != nil {
			t.Fatalf("query %d: %v", i, err)
		}
	}
	if n := fs.starts(); n != 1 {
		t.Fatalf("starts = %d, want 1", n)
	}
	if fs.child(0).closed.Load() {
		t.Fatal("warm child closed between queries")
	}
}

func TestPoolIdleClosesAndRestarts(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, 50*time.Millisecond)
	defer p.Close()
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !fs.child(0).closed.Load() {
		if time.Now().After(deadline) {
			t.Fatal("idle child not closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	if n := fs.starts(); n != 2 {
		t.Fatalf("starts = %d, want 2 (restart after idle)", n)
	}
}

func TestPoolKeyChangeRestarts(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	defer p.Close()
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	changes := []func(o *embed.StartOptions){
		func(o *embed.StartOptions) { o.Model = "/other.gguf" },
		func(o *embed.StartOptions) { o.QueryPrefix = "search_query: " },
		func(o *embed.StartOptions) { o.Threads = 3 },
		func(o *embed.StartOptions) { o.AllowUnsandboxed = true },
		func(o *embed.StartOptions) { o.RepoRoot = "/repo2" },
	}
	for i, change := range changes {
		o := opts()
		change(o)
		if err := query(t, p, o); err != nil {
			t.Fatal(err)
		}
		if n := fs.starts(); n != i+2 {
			t.Fatalf("change %d: starts = %d, want %d", i, n, i+2)
		}
		if !fs.child(i).closed.Load() {
			t.Fatalf("change %d: old child left running", i)
		}
	}
}

func TestPoolBrokenChildRestarts(t *testing.T) {
	for _, reason := range []error{embed.ErrEmbedProtocol, embed.ErrEmbedTimeout} {
		fs := &fakeStarter{}
		p := NewPool(fs.start, time.Hour)
		if err := query(t, p, opts()); err != nil {
			t.Fatal(err)
		}
		fs.child(0).fail(fmt.Errorf("%w: boom", reason))
		if err := query(t, p, opts()); !errors.Is(err, reason) {
			t.Fatalf("want %v, got %v", reason, err)
		}
		if !fs.child(0).closed.Load() {
			t.Fatalf("%v: broken child not closed", reason)
		}
		if err := query(t, p, opts()); err != nil {
			t.Fatalf("%v: query after restart: %v", reason, err)
		}
		if n := fs.starts(); n != 2 {
			t.Fatalf("%v: starts = %d, want 2", reason, n)
		}
		_ = p.Close()
	}
}

// A warm child that died while idle is restarted and the query retried
// once, so the caller never sees the stale child's crash.
func TestPoolRetriesIdleDeath(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	defer p.Close()
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	fs.child(0).fail(fmt.Errorf("%w: exit status 137", embed.ErrEmbedCrashed))
	if err := query(t, p, opts()); err != nil {
		t.Fatalf("idle death not retried: %v", err)
	}
	if n := fs.starts(); n != 2 || !fs.child(0).closed.Load() {
		t.Fatalf("starts = %d, old closed = %v", n, fs.child(0).closed.Load())
	}
	// A crash on a freshly started child is reported, not retried.
	q := &fakeStarter{}
	p2 := NewPool(func(ctx context.Context, o *embed.StartOptions) (embed.Embedder, bool, error) {
		e, u, err := q.start(ctx, o)
		e.(*fakeEmbedder).fail(fmt.Errorf("%w: boom", embed.ErrEmbedCrashed))
		return e, u, err
	}, time.Hour)
	defer p2.Close()
	if err := query(t, p2, opts()); !errors.Is(err, embed.ErrEmbedCrashed) || q.starts() != 1 {
		t.Fatalf("fresh crash: err = %v, starts = %d", err, q.starts())
	}
}

// A per-request error (the child keeps serving) keeps the warm child.
func TestPoolRequestErrorKeepsChild(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	defer p.Close()
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	fs.child(0).fail(&embed.ChildError{Msg: "bad text"})
	if err := query(t, p, opts()); !errors.Is(err, embed.ErrEmbedRequest) {
		t.Fatalf("want embed_request, got %v", err)
	}
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	if n := fs.starts(); n != 1 {
		t.Fatalf("starts = %d, want 1", n)
	}
}

func TestPoolCloseKills(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	if err := query(t, p, opts()); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if !fs.child(0).closed.Load() {
		t.Fatal("Close left the child running")
	}
	if _, _, _, err := p.Embedder(context.Background(), opts()); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("want ErrPoolClosed, got %v", err)
	}
}

// Concurrent tool calls share one child and never overlap on it.
func TestPoolSerializesConcurrentCalls(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	defer p.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			emb, release, _, err := p.Embedder(context.Background(), opts())
			if err != nil {
				errs <- err
				return
			}
			defer release()
			if _, _, err := emb.Embed(context.Background(), embed.KindQuery, []string{"q"}); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if n := fs.starts(); n != 1 {
		t.Fatalf("starts = %d, want 1", n)
	}
	if fs.child(0).overlap.Load() {
		t.Fatal("two Embed calls ran on the child at once")
	}
}

func TestPoolWaitHonorsContext(t *testing.T) {
	fs := &fakeStarter{}
	p := NewPool(fs.start, time.Hour)
	defer p.Close()
	_, release, _, err := p.Embedder(context.Background(), opts())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, _, err := p.Embedder(ctx, opts()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline while leased, got %v", err)
	}
}

func TestOneShotClosesOnRelease(t *testing.T) {
	fs := &fakeStarter{}
	emb, release, _, err := OneShot{Start: fs.start}.Embedder(context.Background(), opts())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := emb.Embed(context.Background(), embed.KindQuery, []string{"q"}); err != nil {
		t.Fatal(err)
	}
	release()
	if !fs.child(0).closed.Load() {
		t.Fatal("one-shot child not closed on release")
	}
}

func TestCheckMode(t *testing.T) {
	for _, m := range []string{ModeLexical, ModeSemantic, ModeHybrid} {
		if err := CheckMode(m, "mode"); err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckMode("fuzzy", "mode"); err == nil || err.Error() != `invalid mode "fuzzy": lexical, semantic or hybrid` {
		t.Fatalf("got %v", err)
	}
}
