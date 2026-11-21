//go:build embednative && cgo

package native

import (
	"math"
	"os"
	"strings"
	"testing"
)

func cosine(a, b []float32) float64 {
	s := 0.0
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

// TestEmbedSmoke loads the CI smoke model (TB_EMBED_MODEL) and checks that
// vectors are normalized, have the model's length, and rank a paraphrase
// above an unrelated text.
func TestEmbedSmoke(t *testing.T) {
	path := os.Getenv("TB_EMBED_MODEL")
	if path == "" {
		t.Skip("TB_EMBED_MODEL not set")
	}
	m, err := Open(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.Dims() < 8 || m.NCtx() < 16 {
		t.Fatalf("dims %d n_ctx %d", m.Dims(), m.NCtx())
	}
	texts := []string{
		"the function refuses source files reached through a symbolic link",
		"symlinked sources are rejected by this check",
		"bananas are a good source of potassium",
	}
	vecs := make([][]float32, len(texts))
	for i, s := range texts {
		v, tr, err := m.Embed(s)
		if err != nil || tr {
			t.Fatalf("%q: %v truncated=%v", s, err, tr)
		}
		if len(v) != m.Dims() || math.Abs(cosine(v, v)-1) > 1e-3 {
			t.Fatalf("%q: len %d norm %f", s, len(v), cosine(v, v))
		}
		vecs[i] = v
	}
	if near, far := cosine(vecs[0], vecs[1]), cosine(vecs[0], vecs[2]); near <= far {
		t.Fatalf("paraphrase %.3f not above unrelated %.3f", near, far)
	}
	if _, tr, err := m.Embed(strings.Repeat("token ", 4*m.NCtx())); err != nil || !tr {
		t.Fatalf("long text: %v truncated=%v", err, tr)
	}
	if _, _, err := m.Embed(""); err != ErrTokenize {
		t.Fatalf("empty text: %v", err)
	}
	m.Close()
	m.Close()
}
