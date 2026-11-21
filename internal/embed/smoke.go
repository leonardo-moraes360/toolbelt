package embed

import (
	"fmt"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

// SmokeResult is what tb __graph-embed --smoke prints.
type SmokeResult struct {
	Engine    string  `json:"engine"`
	Dims      int     `json:"dims"`
	NCtx      int     `json:"n_ctx"`
	Near, Far float64 `json:"-"`
	OK        bool    `json:"ok"`
}

// Smoke checks the model, loads it and embeds three fixed texts: a
// paraphrase must rank above an unrelated text (ADR-018 smoke matrix).
func Smoke(path, sha string, threads int) (*SmokeResult, error) {
	if !native.Available {
		return nil, native.ErrUnavailable
	}
	hashed, err := CheckModel(path, sha)
	if err != nil {
		return nil, err
	}
	m, err := native.Open(path, threads)
	if err != nil {
		return nil, err
	}
	defer m.Close()
	if err := SameModel(path, hashed); err != nil {
		return nil, err
	}
	texts := []string{
		"the function refuses source files reached through a symbolic link",
		"symlinked sources are rejected by this check",
		"bananas are a good source of potassium",
	}
	var vecs [3][]float32
	for i, s := range texts {
		v, _, err := m.Embed(s)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	r := &SmokeResult{Engine: native.Engine(), Dims: m.Dims(), NCtx: m.NCtx(),
		Near: dot(vecs[0], vecs[1]), Far: dot(vecs[0], vecs[2])}
	r.OK = r.Near > r.Far
	if !r.OK {
		return r, fmt.Errorf("embed smoke: paraphrase %.3f not above unrelated %.3f", r.Near, r.Far)
	}
	return r, nil
}

func dot(a, b []float32) float64 {
	s := 0.0
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}
