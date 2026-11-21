package embed

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// StartOptions configure the child; Exe and Args exist for tests.
type StartOptions struct {
	Exe                    string
	Args                   []string
	Model, SHA256          string
	Threads                int
	Timeout                time.Duration // per batch
	AllowUnsandboxed       bool
	QueryPrefix, DocPrefix string
	// RepoRoot, when set, is the repository being indexed: a model inside
	// it is refused, since the repository's author would choose the bytes
	// the native parser reads.
	RepoRoot string
}

const (
	defaultTimeout = 2 * time.Minute
	handshakeMax   = 64 << 10
	// maxDims bounds the handshake so the reply cap stays sane.
	maxDims = 1 << 16
	// requestOverhead covers {"id":…,"kind":"query","texts":[]} around the texts.
	requestOverhead = 64
	// exitGrace is how long a child that closed stdout may take to exit
	// before it is killed.
	exitGrace = 5 * time.Second
)

type lineMsg struct {
	b   []byte
	err error
}

// Client is the parent side of the protocol; it serializes requests.
type Client struct {
	info        Info
	unsandboxed bool
	timeout     time.Duration
	docPrefix   string
	queryPrefix string
	maxLine     int

	reqMu  sync.Mutex // one request in flight
	mu     sync.Mutex
	nextID int64
	broken error

	stdin  *os.File
	stdout *os.File
	lines  chan lineMsg
	quit   chan struct{}
	stop   sync.Once
	cancel context.CancelFunc
	done   chan struct{}
	res    *sandbox.Result
	runErr error
	dir    string
	closed sync.Once
}

var _ Embedder = (*Client)(nil)

// Start checks the model, spawns the sandboxed child and reads its
// handshake. Without a sandbox that denies the network it refuses with
// ErrEmbedUnsandboxed unless AllowUnsandboxed.
func Start(ctx context.Context, o *StartOptions) (*Client, error) {
	if _, err := CheckModel(o.Model, o.SHA256); err != nil {
		return nil, err
	}
	if err := OutsideRepo(o.Model, o.RepoRoot); err != nil {
		return nil, err
	}
	unsandboxed := !sandbox.NetworkDenyAvailable()
	if unsandboxed && !o.AllowUnsandboxed {
		return nil, fmt.Errorf("%w: no Landlock and seccomp sandbox on this host; set graph.embed.allow_unsandboxed to run the embedding child without isolation", ErrEmbedUnsandboxed)
	}
	exe := o.Exe
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return nil, fmt.Errorf("embed: current executable: %w", err)
		}
	}
	exe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("embed: executable: %w", err)
	}
	threads := max(o.Threads, 1)
	args := o.Args
	if args == nil {
		args = []string{"__graph-embed", "--model", o.Model, "--sha256", o.SHA256, "--threads", strconv.Itoa(threads)}
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	c := &Client{
		unsandboxed: unsandboxed, timeout: timeout, docPrefix: o.DocPrefix, queryPrefix: o.QueryPrefix,
		maxLine: handshakeMax, lines: make(chan lineMsg), quit: make(chan struct{}), done: make(chan struct{}),
	}
	if err := c.spawn(exe, args, o); err != nil {
		return nil, err
	}
	if err := c.handshake(ctx, o.SHA256); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) spawn(exe string, args []string, o *StartOptions) (err error) {
	if c.dir, err = os.MkdirTemp("", "tb-embed-"); err != nil {
		return fmt.Errorf("embed: temp dir: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(c.dir)
		}
	}()
	work, tmp, home := filepath.Join(c.dir, "work"), filepath.Join(c.dir, "tmp"), filepath.Join(c.dir, "home")
	for _, d := range []string{work, tmp, home} {
		if err := os.Mkdir(d, 0o700); err != nil {
			return fmt.Errorf("embed: temp dir: %w", err)
		}
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("embed: pipe: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_, _ = inR.Close(), inW.Close()
		return fmt.Errorf("embed: pipe: %w", err)
	}
	sb, err := sandbox.New(sandbox.Config{
		WorkDir:          work,
		ReadOnlyWorkDir:  true,
		PrivateTmp:       tmp,
		Home:             home,
		MinimalDev:       true,
		DenyNetwork:      true,
		AllowUnsandboxed: c.unsandboxed,
		Env:              minimalEnv(),
		AllowedPaths:     []sandbox.PathAccess{{Path: o.Model, ReadOnly: true}, {Path: exe, ReadOnly: true}},
		Stdin:            inR,
		Stdout:           outW,
		MaxStdoutBytes:   math.MaxInt,
		ProcessGroup:     true,
	})
	if err != nil {
		_, _, _, _ = inR.Close(), inW.Close(), outR.Close(), outW.Close()
		return fmt.Errorf("%w: %v", ErrEmbedUnsandboxed, err)
	}
	c.stdin, c.stdout = inW, outR
	runCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go func() {
		c.res, c.runErr = sb.Run(runCtx, exe, args...)
		// Our copies of the child's ends: closing them turns the child's death
		// into EOF on stdout and EPIPE on stdin.
		_, _ = inR.Close(), outW.Close()
		close(c.done)
	}()
	go c.readLoop()
	return nil
}

// minimalEnv keeps only what a process needs to start and format text; the
// sandbox overrides HOME, XDG_* and TMPDIR.
func minimalEnv() []string {
	var out []string
	for _, e := range execenv.Filter(os.Environ(), execenv.Options{}) {
		k, _, _ := strings.Cut(e, "=")
		if k == "PATH" || k == "LANG" || k == "LANGUAGE" || k == "TZ" || strings.HasPrefix(k, "LC_") {
			out = append(out, e)
		}
	}
	return out
}

func (c *Client) readLoop() {
	br := bufio.NewReaderSize(c.stdout, 64<<10)
	for {
		// Wait for the line's first byte before reading the cap: handshake()
		// raises it only after this loop has handed over the handshake line,
		// and reply bytes cannot arrive before the first request, which is
		// sent after that. Early, unsolicited bytes get the smaller cap.
		_, _ = br.Peek(1)
		c.mu.Lock()
		limit := c.maxLine
		c.mu.Unlock()
		b, err := readLine(br, limit)
		select {
		case c.lines <- lineMsg{b: b, err: err}:
		case <-c.quit:
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *Client) handshake(ctx context.Context, sha string) error {
	line, err := c.roundTrip(ctx, nil)
	if err != nil {
		return c.startErr(err)
	}
	var h handshake
	if err := json.Unmarshal(line, &h); err != nil {
		return c.fail(fmt.Errorf("%w: malformed handshake", ErrEmbedProtocol))
	}
	switch {
	case h.V != ProtocolVersion:
		return c.fail(fmt.Errorf("%w: protocol version %d, want %d", ErrEmbedProtocol, h.V, ProtocolVersion))
	case h.Dims <= 0 || h.Dims > maxDims || h.NCtx <= 0 || h.Engine == "":
		return c.fail(fmt.Errorf("%w: invalid handshake (engine %q, dims %d, n_ctx %d)", ErrEmbedProtocol, h.Engine, h.Dims, h.NCtx))
	case h.ModelSHA256 != sha:
		return c.fail(fmt.Errorf("%w: child loaded model %q, want %q", ErrEmbedProtocol, h.ModelSHA256, sha))
	}
	c.info = h.Info
	c.mu.Lock()
	c.maxLine = MaxTexts*((h.Dims*16+2)/3) + 64<<10
	c.mu.Unlock()
	return nil
}

// startErr gives a child that died before its handshake the reason it
// printed, when that reason is one of ours.
func (c *Client) startErr(err error) error {
	if !errors.Is(err, ErrEmbedCrashed) || c.res == nil {
		return err
	}
	// Scan from the end: the runtime may print after the child's own message
	// (a coverage build adds a GOCOVERDIR warning at exit).
	lines := strings.Split(strings.TrimRight(c.res.Output, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		msg := strings.TrimSpace(lines[i])
		for _, known := range []error{native.ErrUnavailable, ErrModelMismatch, ErrModelPath, native.ErrUnsupported, native.ErrModelLoad} {
			code, _, _ := strings.Cut(known.Error(), ":")
			if strings.Contains(msg, code) {
				return fmt.Errorf("%w (child: %s)", known, msg)
			}
		}
	}
	return err
}

// Info is the child's handshake.
func (c *Client) Info() Info { return c.info }

// Unsandboxed reports that the child runs without Landlock and seccomp
// (allow_unsandboxed); callers print the embed_unsandboxed warning.
func (c *Client) Unsandboxed() bool { return c.unsandboxed }

// Embed prefixes the texts, splits them into batches within the protocol
// limits and checks every reply. Any protocol fault, crash or timeout kills
// the child; later calls return the same error.
func (c *Client) Embed(ctx context.Context, kind Kind, texts []string) (vecs [][]float32, truncated []bool, err error) {
	var prefix string
	switch kind {
	case KindDoc:
		prefix = c.docPrefix
	case KindQuery:
		prefix = c.queryPrefix
	default:
		return nil, nil, fmt.Errorf("embed: unknown kind %q", kind)
	}
	c.reqMu.Lock()
	defer c.reqMu.Unlock()
	c.mu.Lock()
	broken := c.broken
	c.mu.Unlock()
	if broken != nil {
		return nil, nil, broken
	}
	batches, err := splitBatches(prefix, texts)
	if err != nil {
		return nil, nil, err
	}
	vecs = make([][]float32, 0, len(texts))
	truncated = make([]bool, 0, len(texts))
	for _, b := range batches {
		v, tr, err := c.embedBatch(ctx, kind, b)
		if err != nil {
			return nil, nil, err
		}
		vecs = append(vecs, v...)
		truncated = append(truncated, tr...)
	}
	return vecs, truncated, nil
}

// splitBatches cuts texts into requests of at most MaxTexts texts whose
// encoded line stays within MaxRequestBytes.
func splitBatches(prefix string, texts []string) ([][]string, error) {
	var out [][]string
	var cur []string
	size := requestOverhead
	for i, t := range texts {
		t = prefix + t
		enc, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		n := len(enc) + 1
		if requestOverhead+n > MaxRequestBytes {
			return nil, fmt.Errorf("embed: text %d is %d bytes encoded, over the %d-byte request limit", i, len(enc), MaxRequestBytes)
		}
		if len(cur) == MaxTexts || size+n > MaxRequestBytes {
			out = append(out, cur)
			cur, size = nil, requestOverhead
		}
		cur = append(cur, t)
		size += n
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out, nil
}

func (c *Client) embedBatch(ctx context.Context, kind Kind, texts []string) (vecs [][]float32, truncated []bool, err error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	req, err := json.Marshal(request{ID: id, Kind: kind, Texts: texts})
	if err != nil {
		return nil, nil, err
	}
	if len(req) > MaxRequestBytes {
		return nil, nil, fmt.Errorf("embed: request of %d bytes over the %d-byte limit", len(req), MaxRequestBytes)
	}
	line, err := c.roundTrip(ctx, append(req, '\n'))
	if err != nil {
		return nil, nil, err
	}
	var rep struct {
		ID        *int64   `json:"id"`
		Vectors   []string `json:"vectors"`
		Truncated []int    `json:"truncated"`
		Error     *string  `json:"error"`
	}
	if err := json.Unmarshal(line, &rep); err != nil {
		return nil, nil, c.fail(fmt.Errorf("%w: malformed reply", ErrEmbedProtocol))
	}
	if rep.ID == nil || *rep.ID != id {
		return nil, nil, c.fail(fmt.Errorf("%w: reply id does not match request %d", ErrEmbedProtocol, id))
	}
	if rep.Error != nil {
		// The text comes from the child: quote it (no terminal escapes) and
		// cap it, under a code of ours so it cannot pose as another reason.
		return nil, nil, &ChildError{Msg: *rep.Error}
	}
	if len(rep.Vectors) != len(texts) {
		return nil, nil, c.fail(fmt.Errorf("%w: %d vectors for %d texts", ErrEmbedProtocol, len(rep.Vectors), len(texts)))
	}
	vecs = make([][]float32, len(texts))
	for i, s := range rep.Vectors {
		if vecs[i], err = decodeVector(s, c.info.Dims); err != nil {
			return nil, nil, c.fail(fmt.Errorf("%w: text %d: %v", ErrEmbedProtocol, i, err))
		}
	}
	truncated = make([]bool, len(texts))
	for _, i := range rep.Truncated {
		if i < 0 || i >= len(texts) || truncated[i] {
			return nil, nil, c.fail(fmt.Errorf("%w: bad truncated index %d", ErrEmbedProtocol, i))
		}
		truncated[i] = true
	}
	return vecs, truncated, nil
}

// roundTrip writes req (if any) and waits for one line, all within the
// per-batch timeout. The write runs aside because a child that stops
// reading would block it on a full pipe.
func (c *Client) roundTrip(ctx context.Context, req []byte) ([]byte, error) {
	c.mu.Lock()
	limit := c.maxLine
	c.mu.Unlock()
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	var werr chan error
	if req != nil {
		werr = make(chan error, 1)
		go func() {
			_, err := c.stdin.Write(req)
			werr <- err
		}()
	}
	lines := c.lines
	var line []byte
	for lines != nil || werr != nil {
		select {
		case err := <-werr:
			werr = nil
			if err != nil {
				return nil, c.exitErr()
			}
		case m := <-lines:
			lines = nil
			switch {
			case errors.Is(m.err, errLineTooLong):
				return nil, c.fail(fmt.Errorf("%w: reply line over %d bytes", ErrEmbedProtocol, limit))
			case m.err != nil:
				return nil, c.exitErr()
			}
			line = m.b
		case <-timer.C:
			return nil, c.fail(fmt.Errorf("%w: no reply within %s", ErrEmbedTimeout, c.timeout))
		case <-ctx.Done():
			return nil, c.fail(ctx.Err())
		}
	}
	return line, nil
}

// fail kills the child and makes err sticky.
func (c *Client) fail(err error) error {
	c.cancel()
	c.stopReader()
	<-c.done
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.broken == nil {
		c.broken = err
	}
	return c.broken
}

// stopReader unblocks readLoop and closes our read end, so the sandbox's
// copy of the child's stdout drains into EPIPE instead of blocking Wait.
func (c *Client) stopReader() {
	c.stop.Do(func() {
		close(c.quit)
		_ = c.stdout.Close()
	})
}

// exitErr waits for a child that closed its stdout and reports how it
// ended.
func (c *Client) exitErr() error {
	select {
	case <-c.done:
	case <-time.After(exitGrace):
		c.cancel()
		<-c.done
	}
	var err error
	switch {
	case c.runErr != nil:
		err = fmt.Errorf("%w: %v", ErrEmbedCrashed, c.runErr)
	case c.res == nil:
		err = fmt.Errorf("%w: no result", ErrEmbedCrashed)
	default:
		err = fmt.Errorf("%w: exit status %d", ErrEmbedCrashed, c.res.ExitCode)
		if msg := lastLine(c.res.Output); msg != "" {
			err = fmt.Errorf("%w: %s", err, msg)
		}
	}
	return c.fail(err)
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 512 {
		s = s[:512]
	}
	return s
}

// Close ends the child: EOF on its stdin, then a kill if it lingers.
func (c *Client) Close() error {
	c.closed.Do(func() {
		_ = c.stdin.Close()
		select {
		case <-c.done:
		case <-time.After(exitGrace):
		}
		c.cancel()
		c.stopReader()
		<-c.done
		_ = os.RemoveAll(c.dir)
		c.mu.Lock()
		if c.broken == nil {
			c.broken = errors.New("embed: client closed")
		}
		c.mu.Unlock()
	})
	return nil
}
