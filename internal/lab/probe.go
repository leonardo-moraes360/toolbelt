package lab

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// Availability reports whether a tier can run here and, when it cannot, a
// specific remediation the caller surfaces instead of degrading isolation.
type Availability struct {
	Tier        Tier   `json:"tier"`
	Available   bool   `json:"available"`
	Remediation string `json:"remediation,omitempty"`
}

// podmanProbe is the podman availability check; overridable in tests.
var podmanProbe = probePodmanReal

// landlockProbe is the Landlock check behind ProbeSandbox; tests replace it to
// reach the fail-closed path on kernels that have Landlock.
var landlockProbe = sandbox.CheckLandlockSupport

// ProbeSandbox reports whether the sandbox tier can run. It requires Landlock
// on Linux; elsewhere the tier is unavailable (fail-closed).
func ProbeSandbox() Availability {
	a := Availability{Tier: TierSandbox}
	if runtime.GOOS != "linux" {
		a.Remediation = "the sandbox tier requires Linux with Landlock; this host is " + runtime.GOOS
		return a
	}
	ok, _, err := landlockProbe()
	if ok {
		a.Available = true
		return a
	}
	a.Remediation = "Landlock is unavailable on this kernel"
	if err != nil {
		a.Remediation += " (" + err.Error() + ")"
	}
	a.Remediation += "; a sandbox-tier lab needs Landlock (Linux 5.13+)"
	return a
}

// ProbePodman reports whether the rootless-Podman topology tier can run.
func ProbePodman() Availability {
	return podmanProbe()
}

func probePodmanReal() Availability {
	a := Availability{Tier: TierPodman}
	bin, err := exec.LookPath("podman")
	if err != nil {
		a.Remediation = "podman not found on PATH; install rootless Podman (https://podman.io) " +
			"to run a topology lab, or use --tier sandbox for a single-step lab"
		return a
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// `podman info` confirms the binary is usable and rootless. We do not trust
	// a mere binary on PATH: a broken or root-only install must fail closed.
	out, err := exec.CommandContext(ctx, bin, "info", "--format", "{{.Host.Security.Rootless}}").CombinedOutput()
	if err != nil {
		a.Remediation = "`podman info` failed (" + firstLine(string(out)) + "); " +
			"ensure rootless Podman is configured, or use --tier sandbox"
		return a
	}
	if strings.TrimSpace(string(out)) != "true" {
		a.Remediation = "Podman is not running rootless; tb lab requires rootless, daemonless Podman. " +
			"Configure rootless Podman, or use --tier sandbox"
		return a
	}
	a.Available = true
	return a
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
