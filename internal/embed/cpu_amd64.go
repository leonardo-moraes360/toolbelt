//go:build amd64

package embed

import (
	"fmt"

	"golang.org/x/sys/cpu"
)

// CheckCPU refuses CPUs below the AVX2 tier the amd64 ggml-cpu kernels are
// built for (ADR-019): running them there dies with SIGILL. HasAVX and
// HasAVX2 already include the OSXSAVE/XCR0 check that the OS saves AVX
// state; BMI2 is checked too because hypervisors can mask it while
// exposing AVX2.
func CheckCPU() error {
	var missing []string
	for _, f := range []struct {
		name string
		ok   bool
	}{{"AVX", cpu.X86.HasAVX}, {"AVX2", cpu.X86.HasAVX2}, {"FMA", cpu.X86.HasFMA}, {"F16C", cpu.X86.HasF16C}, {"BMI2", cpu.X86.HasBMI2}} {
		if !f.ok {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: the embedding engine needs an x86-64-v3 CPU (missing %v); every other tb command still works", ErrEmbedCPU, missing)
	}
	return nil
}
