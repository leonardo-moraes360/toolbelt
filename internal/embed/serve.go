package embed

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

// ProtocolVersion is the "v" of the handshake.
const ProtocolVersion = 1

// handshake is the child's first line (spec, Protocol).
type handshake struct {
	V int `json:"v"`
	Info
}

type request struct {
	ID    int64    `json:"id"`
	Kind  Kind     `json:"kind"`
	Texts []string `json:"texts"`
}

type reply struct {
	ID        int64    `json:"id"`
	Vectors   []string `json:"vectors,omitempty"`
	Truncated []int    `json:"truncated,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// engine is what the serve loop needs from native.Model; tests run the same
// loop over a fake engine.
type engine interface {
	Embed(text string) (vec []float32, truncated bool, err error)
}

var (
	errLineTooLong = errors.New("line too long")
	// errLineCut is errLineTooLong when the rest of the line is still unread.
	errLineCut = fmt.Errorf("%w (rest unread)", errLineTooLong)
)

// Serve is the child side of tb __graph-embed: it re-checks the model the
// parent already checked (the child must not trust its arguments), loads it,
// and answers requests until in reaches EOF.
func Serve(ctx context.Context, in io.Reader, out io.Writer, model, sha string, threads int) error {
	if !native.Available {
		return native.ErrUnavailable
	}
	hashed, err := CheckModel(model, sha)
	if err != nil {
		return err
	}
	m, err := native.Open(model, threads)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := SameModel(model, hashed); err != nil {
		return err
	}
	info := Info{Engine: native.EngineName, Commit: native.EngineCommit, ModelSHA256: sha, Dims: m.Dims(), NCtx: m.NCtx()}
	return serve(ctx, in, out, info, m)
}

func serve(ctx context.Context, in io.Reader, out io.Writer, info Info, eng engine) error {
	br := bufio.NewReaderSize(in, 64<<10)
	w := bufio.NewWriter(out)
	if err := writeLine(w, handshake{V: ProtocolVersion, Info: info}); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := readLine(br, MaxRequestBytes)
		var rep reply
		switch {
		case errors.Is(err, errLineTooLong):
			if errors.Is(err, errLineCut) {
				if err := skipLine(br); err != nil && !errors.Is(err, io.EOF) {
					return err
				}
			}
			rep = reply{Error: fmt.Sprintf("embed_request: request over %d bytes", MaxRequestBytes)}
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		default:
			rep = handle(line, info.Dims, eng)
		}
		if err := writeLine(w, rep); err != nil {
			return err
		}
	}
}

func handle(line []byte, dims int, eng engine) reply {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return reply{Error: "embed_request: malformed request"}
	}
	rep := reply{ID: req.ID}
	switch {
	case req.Kind != KindDoc && req.Kind != KindQuery:
		rep.Error = fmt.Sprintf("embed_request: unknown kind %q", req.Kind)
		return rep
	case len(req.Texts) > MaxTexts:
		rep.Error = fmt.Sprintf("embed_request: %d texts, over %d", len(req.Texts), MaxTexts)
		return rep
	}
	rep.Vectors = make([]string, 0, len(req.Texts))
	for i, t := range req.Texts {
		vec, tr, err := eng.Embed(t)
		if err != nil {
			return reply{ID: req.ID, Error: err.Error()}
		}
		if len(vec) != dims {
			return reply{ID: req.ID, Error: fmt.Sprintf("embed: engine returned %d dims, want %d", len(vec), dims)}
		}
		rep.Vectors = append(rep.Vectors, encodeVector(vec))
		if tr {
			rep.Truncated = append(rep.Truncated, i)
		}
	}
	return rep
}

func writeLine(w *bufio.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	return w.Flush()
}

// readLine returns the next line without its newline, refusing to buffer
// more than limit bytes of it: a peer cannot make the reader grow without
// bound. A final line without newline is io.ErrUnexpectedEOF.
func readLine(br *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := br.ReadSlice('\n')
		if len(line)+len(chunk) > limit+1 {
			if err == nil {
				return nil, errLineTooLong
			}
			return nil, errLineCut
		}
		line = append(line, chunk...)
		switch {
		case err == nil:
			return line[:len(line)-1], nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return nil, io.ErrUnexpectedEOF
		default:
			return nil, err
		}
	}
}

func skipLine(br *bufio.Reader) error {
	for {
		_, err := br.ReadSlice('\n')
		if !errors.Is(err, bufio.ErrBufferFull) {
			return err
		}
	}
}

func encodeVector(v []float32) string {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// decodeVector accepts only exactly dims finite floats.
func decodeVector(s string, dims int) ([]float32, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("vector is not base64: %v", err)
	}
	if len(b) != 4*dims {
		return nil, fmt.Errorf("vector has %d bytes, want %d", len(b), 4*dims)
	}
	v := make([]float32, dims)
	for i := range v {
		f := math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			return nil, fmt.Errorf("vector component %d is not finite", i)
		}
		v[i] = f
	}
	return v, nil
}
