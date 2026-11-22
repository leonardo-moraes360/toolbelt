package embed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// childMsg is the raw text of a child error reply (fake modes use it to
// return their logs).
func childMsg(t *testing.T, err error) string {
	t.Helper()
	var ce *ChildError
	if !errors.As(err, &ce) {
		t.Fatalf("not a child error reply: %v", err)
	}
	return ce.Msg
}

// modelFile writes a model file CheckModel accepts and returns its path
// and sha256.
func modelFile(t *testing.T) (model, sha string) {
	t.Helper()
	p := filepath.Join(tempDir(t), "m.gguf")
	if err := os.WriteFile(p, []byte("GGUF fake model"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("GGUF fake model"))
	return p, hex.EncodeToString(sum[:])
}

func fakeOptions(t *testing.T, mode string, extra ...string) *StartOptions {
	t.Helper()
	model, sha := modelFile(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &StartOptions{
		Exe:   exe,
		Args:  append([]string{fakeArg, mode, sha}, extra...),
		Model: model, SHA256: sha,
		Timeout:          10 * time.Second,
		AllowUnsandboxed: true,
		DocPrefix:        "doc: ", QueryPrefix: "query: ",
	}
}

func startFake(t *testing.T, mode string, extra ...string) *Client {
	t.Helper()
	c, err := Start(context.Background(), fakeOptions(t, mode, extra...))
	if err != nil {
		t.Fatalf("Start(%s): %v", mode, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClientHappyPath(t *testing.T) {
	c := startFake(t, "ok")
	t.Run("info", func(t *testing.T) { checkFakeInfo(t, c) })
	t.Run("doc", func(t *testing.T) { checkDocEmbed(t, c) })
	t.Run("query", func(t *testing.T) {
		vecs, _, err := c.Embed(context.Background(), KindQuery, []string{"q"})
		if err != nil || vecs[0][0] != float32(len("query: q")) {
			t.Fatalf("query: %v %v", vecs, err)
		}
	})
	t.Run("edge", func(t *testing.T) { checkEmbedEdges(t, c) })
}

func checkFakeInfo(t *testing.T, c *Client) {
	t.Helper()
	info := c.Info()
	if info.Dims != fakeDims || info.NCtx != 512 || info.Engine != "fake" || len(info.ModelSHA256) != 64 {
		t.Fatalf("info = %+v", info)
	}
	if got, want := c.Unsandboxed(), !sandbox.NetworkDenyAvailable(); got != want {
		t.Fatalf("Unsandboxed = %v, want %v", got, want)
	}
}

func checkDocEmbed(t *testing.T, c *Client) {
	t.Helper()
	vecs, tr, err := c.Embed(context.Background(), KindDoc, []string{"a", "LONG text"})
	if err != nil {
		t.Fatal(err)
	}
	// The fake vector starts at len(text): the parent added "doc: ".
	if len(vecs) != 2 || vecs[0][0] != float32(len("doc: a")) || vecs[1][3] != float32(len("doc: LONG text")+3) {
		t.Fatalf("vecs = %v", vecs)
	}
	if tr[0] || !tr[1] {
		t.Fatalf("truncated = %v", tr)
	}
}

func checkEmbedEdges(t *testing.T, c *Client) {
	t.Helper()
	if v, tr, err := c.Embed(context.Background(), KindDoc, nil); err != nil || len(v) != 0 || len(tr) != 0 {
		t.Fatalf("empty: %v %v %v", v, tr, err)
	}
	if _, _, err := c.Embed(context.Background(), "other", []string{"x"}); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

// Regression: the read loop picked up the handshake cap before the
// handshake raised it, so the first full batch of a real model failed.
func TestClientFullBatchOfWideVectors(t *testing.T) {
	c := startFake(t, "wide")
	for round := 0; round < 3; round++ {
		texts := make([]string, MaxTexts)
		for i := range texts {
			texts[i] = fmt.Sprintf("text %d of round %d", i, round)
		}
		vecs, _, err := c.Embed(context.Background(), KindDoc, texts)
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		if len(vecs) != MaxTexts || len(vecs[0]) != wideDims {
			t.Fatalf("round %d: %d vectors of %d dims", round, len(vecs), len(vecs[0]))
		}
	}
}

func TestClientBatches(t *testing.T) {
	c := startFake(t, "record")
	texts := make([]string, 150)
	for i := range texts {
		texts[i] = fmt.Sprintf("t%d", i)
	}
	big := strings.Repeat("b", 600<<10)
	texts = append(texts, big, big, big)
	vecs, _, err := c.Embed(context.Background(), KindDoc, texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("%d vectors for %d texts", len(vecs), len(texts))
	}
	_, _, err = c.Embed(context.Background(), KindQuery, []string{"__dump__"})
	if err == nil {
		t.Fatal("dump returned no log")
	}
	var log []struct {
		Kind  Kind
		N     int
		Bytes int
		First string
		Small []string
	}
	if err := json.Unmarshal([]byte(childMsg(t, err)), &log); err != nil {
		t.Fatalf("dump: %v", err)
	}
	var sizes []int
	for _, e := range log {
		sizes = append(sizes, e.N)
		if e.Kind != KindDoc || e.Bytes > MaxRequestBytes || !strings.HasPrefix(e.First, "doc: ") {
			t.Fatalf("batch %+v", e)
		}
	}
	// 150 small texts in 64+64+22 (the first big one joins the third), then
	// one big text per request because two exceed 1 MiB.
	if fmt.Sprint(sizes) != "[64 64 23 1 1]" {
		t.Fatalf("batch sizes = %v", sizes)
	}
	if log[0].Small[0] != "doc: t0" {
		t.Fatalf("first text = %q", log[0].Small[0])
	}
	if _, _, err := c.Embed(context.Background(), KindDoc, []string{strings.Repeat("x", MaxRequestBytes)}); err == nil {
		t.Fatal("text over the request limit accepted")
	}
}

// Acceptance 13: each failure mode returns its reason and kills the child.
func TestClientFailures(t *testing.T) {
	for mode, want := range map[string]error{
		"malformed": ErrEmbedProtocol,
		"oversize":  ErrEmbedProtocol,
		"wrongid":   ErrEmbedProtocol,
		"wronglen":  ErrEmbedProtocol,
		"nan":       ErrEmbedProtocol,
		"badtrunc":  ErrEmbedProtocol,
		"crash":     ErrEmbedCrashed,
		"hang":      ErrEmbedTimeout,
	} {
		t.Run(mode, func(t *testing.T) {
			c, err := Start(context.Background(), fakeOptions(t, mode))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			// The handshake keeps the roomy default: under ASan or -race the
			// child's startup alone can exceed a second. Only the batch
			// under test gets the short timeout.
			c.timeout = time.Second
			start := time.Now()
			_, _, err = c.Embed(context.Background(), KindDoc, []string{"a", "b"})
			if !errors.Is(err, want) {
				t.Fatalf("err = %v, want %v", err, want)
			}
			if time.Since(start) > 8*time.Second {
				t.Fatalf("failure took %s", time.Since(start))
			}
			if mode == "crash" && !strings.Contains(err.Error(), "exit status 3") {
				t.Fatalf("crash error lacks the status: %v", err)
			}
			select {
			case <-c.done:
			default:
				t.Fatal("child still running after the failure")
			}
			if _, _, err2 := c.Embed(context.Background(), KindDoc, []string{"a"}); !errors.Is(err2, want) {
				t.Fatalf("second call: %v", err2)
			}
		})
	}
}

func TestClientErrorReply(t *testing.T) {
	c := startFake(t, "error")
	_, _, err := c.Embed(context.Background(), KindDoc, []string{"a"})
	if !errors.Is(err, ErrEmbedRequest) || childMsg(t, err) != "embed: evaluation failed" {
		t.Fatalf("err = %v", err)
	}
	long := &ChildError{Msg: "vectors_disabled\x1b[2J" + strings.Repeat("x", 2000)}
	if s := long.Error(); !strings.HasPrefix(s, "embed_request: child replied ") || strings.Contains(s, "\x1b") || len(s) > 600 {
		t.Fatalf("child error not quoted/capped: %q", s)
	}
	// An error reply is not a protocol fault: the child keeps serving.
	if _, _, err := c.Embed(context.Background(), KindDoc, []string{"a"}); err == nil || errors.Is(err, ErrEmbedProtocol) {
		t.Fatalf("second err = %v", err)
	}
}

func TestClientContextCancel(t *testing.T) {
	c := startFake(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, _, err := c.Embed(ctx, KindDoc, []string{"a"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestClientStartFailures(t *testing.T) {
	o := fakeOptions(t, "unavailable")
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrEmbedUnavailable) {
		t.Fatalf("unavailable: %v", err)
	}
	o = fakeOptions(t, "unavailable-noisy")
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrEmbedUnavailable) {
		t.Fatalf("unavailable with trailing noise: %v", err)
	}
	o = fakeOptions(t, "cpu")
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrEmbedCPU) {
		t.Fatalf("cpu: %v", err)
	}
	o = fakeOptions(t, "badsha")
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrEmbedProtocol) {
		t.Fatalf("badsha: %v", err)
	}
	o = fakeOptions(t, "ok")
	o.SHA256 = strings.Repeat("a", 64)
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrModelMismatch) {
		t.Fatalf("model mismatch: %v", err)
	}
	o = fakeOptions(t, "ok")
	link := filepath.Join(filepath.Dir(o.Model), "link.gguf")
	if err := os.Symlink(o.Model, link); err != nil {
		t.Fatal(err)
	}
	o.Model = link
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrModelPath) {
		t.Fatalf("symlink: %v", err)
	}
}

func TestClientRefusesUnsandboxed(t *testing.T) {
	restore := sandbox.OverrideNetworkDenyProbe(func() bool { return false })
	defer restore()
	o := fakeOptions(t, "ok")
	o.AllowUnsandboxed = false
	if _, err := Start(context.Background(), o); !errors.Is(err, ErrEmbedUnsandboxed) {
		t.Fatalf("err = %v", err)
	}
	o.AllowUnsandboxed = true
	c, err := Start(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !c.Unsandboxed() {
		t.Fatal("Unsandboxed = false")
	}
	if _, _, err := c.Embed(context.Background(), KindDoc, []string{"a"}); err != nil {
		t.Fatal(err)
	}
}

func TestClientCloseRemovesTemp(t *testing.T) {
	c := startFake(t, "ok")
	dir := c.dir
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temp dir left: %v", err)
	}
	if _, _, err := c.Embed(context.Background(), KindDoc, []string{"a"}); err == nil {
		t.Fatal("Embed after Close succeeded")
	}
}

func TestServeLoop(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	var in bytes.Buffer
	in.WriteString(`{"id":1,"kind":"doc","texts":["x","LONG"]}` + "\n")
	in.WriteString("garbage\n")
	in.WriteString(`{"id":3,"kind":"nope","texts":[]}` + "\n")
	in.WriteString(`{"id":4,"kind":"doc","texts":[` + strings.TrimSuffix(strings.Repeat(`"a",`, MaxTexts+1), ",") + "]}\n")
	in.WriteString(`{"id":5,"kind":"doc","texts":["` + strings.Repeat("y", MaxRequestBytes) + "\"]}\n")
	in.WriteString(`{"id":6,"kind":"query","texts":["z"]}` + "\n")
	var out bytes.Buffer
	if err := serve(context.Background(), &in, &out, fakeInfo(sha), fakeEngine{}); err != nil {
		t.Fatal(err)
	}
	reps := parseServeOutput(t, out.String(), sha)
	if reps[0].ID != 1 || len(reps[0].Vectors) != 2 || fmt.Sprint(reps[0].Truncated) != "[1]" {
		t.Fatalf("reply 1 = %+v", reps[0])
	}
	if v, err := decodeVector(reps[0].Vectors[0], fakeDims); err != nil || v[0] != 1 || v[3] != 4 {
		t.Fatalf("vector = %v %v", v, err)
	}
	for i, id := range []int64{0, 3, 4, 0} {
		if reps[i+1].ID != id || reps[i+1].Error == "" {
			t.Fatalf("reply %d = %+v", i+2, reps[i+1])
		}
	}
	if reps[5].ID != 6 || len(reps[5].Vectors) != 1 {
		t.Fatalf("reply 6 = %+v", reps[5])
	}
}

// parseServeOutput checks the handshake line and decodes the six replies
// TestServeLoop expects after it.
func parseServeOutput(t *testing.T, out, sha string) []reply {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 7 {
		t.Fatalf("%d lines: %q", len(lines), out)
	}
	var h handshake
	if err := json.Unmarshal([]byte(lines[0]), &h); err != nil || h.V != 1 || h.Dims != fakeDims || h.ModelSHA256 != sha {
		t.Fatalf("handshake %q", lines[0])
	}
	var reps []reply
	for _, l := range lines[1:] {
		var r reply
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatal(err)
		}
		reps = append(reps, r)
	}
	return reps
}

func TestServeUnavailable(t *testing.T) {
	if native.Available {
		t.Skip("native engine built in")
	}
	model, sha := modelFile(t)
	err := Serve(context.Background(), strings.NewReader(""), &bytes.Buffer{}, model, sha, 1)
	if !errors.Is(err, ErrEmbedUnavailable) {
		t.Fatalf("err = %v", err)
	}
}
