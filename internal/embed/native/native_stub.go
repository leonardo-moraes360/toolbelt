//go:build !embednative || !cgo

package native

// Available reports whether this binary carries the native engine.
const Available = false

// Model is unavailable in builds without the embednative tag.
type Model struct{}

// Open always fails with ErrUnavailable in this build.
func Open(string, int) (*Model, error) { return nil, ErrUnavailable }

// Dims, NCtx, Embed and Close exist so callers compile in every build.
func (m *Model) Dims() int                             { return 0 }
func (m *Model) NCtx() int                             { return 0 }
func (m *Model) Embed(string) ([]float32, bool, error) { return nil, false, ErrUnavailable }
func (m *Model) Close()                                {}
