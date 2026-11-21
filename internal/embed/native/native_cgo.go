//go:build embednative && cgo

package native

/*
#cgo CFLAGS: -std=c11 -O2 -Wall -fstack-protector-strong -I${SRCDIR}/../../../third_party/llama.cpp/include -I${SRCDIR}/../../../third_party/llama.cpp/ggml/include
#cgo linux CFLAGS: -D_FORTIFY_SOURCE=2
#cgo windows CFLAGS: -fno-stack-protector
// -fcf-protection is not in cgo's flag allowlist. The libraries get it from
// scripts/build-native.sh; the binary is not CET-marked either way, since
// the Go code is not built with it.
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/build/linux_amd64/lib
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/build/linux_arm64/lib
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/build/darwin_amd64/lib -F${SRCDIR}/build/darwin_amd64/lib/Frameworks
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/build/darwin_arm64/lib -F${SRCDIR}/build/darwin_arm64/lib/Frameworks
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/build/windows_amd64/lib
#cgo windows,arm64 LDFLAGS: -L${SRCDIR}/build/windows_arm64/lib
#cgo LDFLAGS: -lllama -lggml -lggml-cpu -lggml-base -lm
#cgo linux LDFLAGS: -lpthread
#include <stdlib.h>
#include "tbembed.h"
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// Available reports whether this binary carries the native engine.
const Available = true

// Model is a loaded embedding model. It is not safe for concurrent use.
type Model struct {
	m    *C.tbe_model
	dims int
	nCtx int
}

func statusErr(rc C.int) error {
	switch rc {
	case C.TBE_OK:
		return nil
	case C.TBE_ARGS:
		return ErrArgs
	case C.TBE_MODEL_LOAD:
		return ErrModelLoad
	case C.TBE_UNSUPPORTED:
		return ErrUnsupported
	case C.TBE_TOKENIZE:
		return ErrTokenize
	case C.TBE_NOMEM:
		return ErrNoMem
	}
	return ErrEval
}

// Open loads the GGUF model at path. The caller has already checked the
// path and the model hash (ADR-018 §7).
func Open(path string, threads int) (*Model, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var m *C.tbe_model
	if err := statusErr(C.tbe_open(cpath, C.int32_t(threads), &m)); err != nil {
		return nil, err
	}
	mod := &Model{m: m, dims: int(C.tbe_dims(m)), nCtx: int(C.tbe_n_ctx(m))}
	runtime.SetFinalizer(mod, (*Model).Close)
	return mod, nil
}

// Dims is the vector length; NCtx the token limit per text.
func (m *Model) Dims() int { return m.dims }
func (m *Model) NCtx() int { return m.nCtx }

// Embed returns the L2-normalized vector of text and whether it was cut
// to NCtx tokens.
func (m *Model) Embed(text string) (vec []float32, truncated bool, err error) {
	if m.m == nil {
		return nil, false, ErrArgs
	}
	if text == "" {
		return nil, false, ErrTokenize
	}
	out := make([]float32, m.dims)
	ctext := C.CBytes([]byte(text))
	defer C.free(ctext)
	var tr C.int
	rc := C.tbe_embed(m.m, (*C.char)(ctext), C.size_t(len(text)), (*C.float)(unsafe.Pointer(&out[0])), C.size_t(len(out)), &tr)
	// m must stay reachable until the C call returns, or its finalizer
	// could free the model while tbe_embed runs.
	runtime.KeepAlive(m)
	runtime.KeepAlive(out)
	if err := statusErr(rc); err != nil {
		return nil, false, err
	}
	return out, tr != 0, nil
}

// Close frees the model; it is safe to call twice.
func (m *Model) Close() {
	if m.m != nil {
		C.tbe_close(m.m)
		m.m = nil
	}
	runtime.SetFinalizer(m, nil)
}
