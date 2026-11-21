package embed

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake child: `embed.test __fake-embed <mode>
// <sha> [args…]` speaks the protocol without native code.
const fakeArg = "__fake-embed"

const (
	fakeDims = 4
	wideDims = 384
)

// fakeModes beyond the generic ones (hostile lives in the linux file).
var fakeModes = map[string]func(sha string, args []string){}

func TestMain(m *testing.M) {
	if len(os.Args) >= 4 && os.Args[1] == fakeArg {
		runFake(os.Args[2], os.Args[3], os.Args[4:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type fakeEngine struct{}

// Embed gives text-dependent finite vectors; "LONG" marks a truncation.
func (fakeEngine) Embed(t string) ([]float32, bool, error) {
	v := make([]float32, fakeDims)
	for i := range v {
		v[i] = float32(len(t) + i)
	}
	return v, strings.Contains(t, "LONG"), nil
}

type funcEngine func(string) ([]float32, bool, error)

func (f funcEngine) Embed(t string) ([]float32, bool, error) { return f(t) }

func fakeInfo(sha string) Info {
	return Info{Engine: "fake", Commit: "c0ffee", ModelSHA256: sha, Dims: fakeDims, NCtx: 512}
}

func runFake(mode, sha string, args []string) {
	if f, ok := fakeModes[mode]; ok {
		f(sha, args)
		return
	}
	ctx := context.Background()
	switch mode {
	case "ok":
		_ = serve(ctx, os.Stdin, os.Stdout, fakeInfo(sha), fakeEngine{})
	case "crash":
		_ = serve(ctx, os.Stdin, os.Stdout, fakeInfo(sha), funcEngine(func(string) ([]float32, bool, error) {
			fmt.Fprintln(os.Stderr, "fake: boom")
			os.Exit(3)
			return nil, false, nil
		}))
	case "hang":
		_ = serve(ctx, os.Stdin, os.Stdout, fakeInfo(sha), funcEngine(func(string) ([]float32, bool, error) {
			time.Sleep(time.Hour)
			return nil, false, nil
		}))
	case "error":
		_ = serve(ctx, os.Stdin, os.Stdout, fakeInfo(sha), funcEngine(func(string) ([]float32, bool, error) {
			return nil, false, errors.New("embed: evaluation failed")
		}))
	case "unavailable":
		fmt.Fprintln(os.Stderr, "tb __graph-embed: embed_unavailable: this tb was built without the native engine")
		os.Exit(2)
	case "wide":
		// A real model's width: 64 vectors of 384 dims exceed the 64 KiB
		// handshake cap, so the reply cap must already be the per-dims one.
		info := fakeInfo(sha)
		info.Dims = wideDims
		_ = serve(ctx, os.Stdin, os.Stdout, info, funcEngine(func(t string) ([]float32, bool, error) {
			v := make([]float32, wideDims)
			v[len(t)%wideDims] = 1
			return v, false, nil
		}))
	case "badsha":
		_ = serve(ctx, os.Stdin, os.Stdout, fakeInfo(strings.Repeat("0", 64)), fakeEngine{})
	default:
		rawFake(mode, sha)
	}
}

// rawFake answers each request with a reply broken the way mode says, or,
// for record, a correct reply plus a log it returns on a "__dump__" text.
func rawFake(mode, sha string) {
	w := bufio.NewWriter(os.Stdout)
	_ = writeLine(w, handshake{V: ProtocolVersion, Info: fakeInfo(sha)})
	br := bufio.NewReaderSize(os.Stdin, 1<<16)
	var log []request
	for {
		line, err := readLine(br, MaxRequestBytes)
		if err != nil {
			return
		}
		var req request
		_ = json.Unmarshal(line, &req)
		good := handle(line, fakeDims, fakeEngine{})
		switch mode {
		case "malformed":
			_, _ = w.WriteString("{not json\n")
			_ = w.Flush()
		case "oversize":
			_, _ = w.WriteString(`{"id":` + fmt.Sprint(req.ID) + `,"error":"` + strings.Repeat("x", 200<<10) + "\"}\n")
			_ = w.Flush()
		case "wrongid":
			good.ID++
			_ = writeLine(w, good)
		case "wronglen":
			good.Vectors[0] = encodeVector(make([]float32, fakeDims-1))
			_ = writeLine(w, good)
		case "nan":
			good.Vectors[0] = encodeVector([]float32{1, float32(math.NaN()), 0, 0})
			_ = writeLine(w, good)
		case "badtrunc":
			good.Truncated = []int{len(req.Texts)}
			_ = writeLine(w, good)
		case "record":
			if len(req.Texts) == 1 && strings.HasSuffix(req.Texts[0], "__dump__") {
				type entry struct {
					Kind  Kind     `json:"kind"`
					N     int      `json:"n"`
					Bytes int      `json:"bytes"`
					First string   `json:"first"`
					Small []string `json:"small,omitempty"`
				}
				var out []entry
				for _, r := range log {
					e := entry{Kind: r.Kind, N: len(r.Texts)}
					for _, t := range r.Texts {
						e.Bytes += len(t)
					}
					e.First = r.Texts[0]
					if len(e.First) > 40 {
						e.First = e.First[:40]
					}
					if e.Bytes < 4096 {
						e.Small = r.Texts
					}
					out = append(out, e)
				}
				b, _ := json.Marshal(out)
				_ = writeLine(w, reply{ID: req.ID, Error: string(b)})
				continue
			}
			log = append(log, req)
			_ = writeLine(w, good)
		default:
			return
		}
	}
}
