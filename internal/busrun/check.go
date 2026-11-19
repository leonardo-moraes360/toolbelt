package busrun

import (
	"os/exec"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

func lookPath(p string) (string, error) { return exec.LookPath(p) }

// sandboxCheck fails closed without Landlock unless --allow-unsandboxed.
func sandboxCheck(h *Harness) error {
	_, err := sandbox.New(sandbox.Config{WorkDir: h.root, AllowUnsandboxed: h.opts.AllowUnsandboxed})
	return err
}

// landlockOn reports whether the sandbox really isolates (tests).
func landlockOn() bool {
	ok, _, err := sandbox.CheckLandlockSupport()
	return ok && err == nil
}
