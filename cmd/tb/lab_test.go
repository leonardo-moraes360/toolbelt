package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// labRepo makes a temp repo (with .toolbelt) and an isolated lab state dir,
// returning the repo dir and an env slice for the tb subprocess.
func labRepo(t *testing.T) (dir string, env []string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	env = append(os.Environ(), "TB_LAB_STATE_DIR="+t.TempDir())
	return dir, env
}

func runLab(t *testing.T, tbBin, dir string, env []string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(tbBin, append([]string{"lab"}, args...)...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

func TestLabLifecycleCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir, env := labRepo(t)

	// new
	out, err := runLab(t, tbBin, dir, env, "new", "demo", "--class", "sql-injection")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "labs", "demo", "lab.yaml")); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}

	// up (sandbox)
	if out, err := runLab(t, tbBin, dir, env, "up", "demo"); err != nil {
		t.Fatalf("up: %v\n%s", err, out)
	}

	// run → verdict
	out, err = runLab(t, tbBin, dir, env, "run", "demo", "--json")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	var v struct {
		Attack, Defense, MatchedCheck string
	}
	if jerr := json.Unmarshal(stripNotice(out), &v); jerr != nil {
		t.Fatalf("run json: %v\n%s", jerr, out)
	}
	if v.Attack != "reproduced" || v.Defense != "detected" {
		t.Errorf("verdict = attack=%s defense=%s, want reproduced/detected", v.Attack, v.Defense)
	}

	// down + verify clean
	if out, err := runLab(t, tbBin, dir, env, "down", "demo"); err != nil {
		t.Fatalf("down: %v\n%s", err, out)
	}
	out, err = runLab(t, tbBin, dir, env, "verify", "--json")
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"clean":true`) {
		t.Errorf("verify not clean after down: %s", out)
	}
}

// withoutPodman points PATH at a directory holding only git, so tb finds no
// podman whether or not the machine has one (runners ship it). Without this
// the fail-closed path never ran on CI.
func withoutPodman(t *testing.T, env []string) []string {
	t.Helper()
	bin := t.TempDir()
	if git, err := exec.LookPath("git"); err == nil {
		if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
			t.Skipf("cannot stub PATH: %v", err)
		}
	}
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "PATH=") {
			out = append(out, e)
		}
	}
	return append(out, "PATH="+bin)
}

func TestLabUpPodmanFailsClosedCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir, env := labRepo(t)
	env = withoutPodman(t, env)
	if out, err := runLab(t, tbBin, dir, env, "new", "demo", "--class", "sql-injection"); err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	out, err := runLab(t, tbBin, dir, env, "up", "demo", "--tier", "podman")
	if err == nil {
		t.Fatalf("up --tier podman should fail closed without podman\n%s", out)
	}
	if !strings.Contains(string(out), "unavailable") {
		t.Errorf("fail-closed message missing remediation: %s", out)
	}
	// Nothing provisioned.
	if lst, _ := runLab(t, tbBin, dir, env, "verify", "--json"); !strings.Contains(string(lst), `"provisioned":null`) && !strings.Contains(string(lst), `"provisioned":[]`) {
		t.Errorf("podman fail-closed must provision nothing: %s", lst)
	}
}

// stripNotice drops the one-time authorized-use notice line that the lab gate
// prints to stderr (merged into CombinedOutput) before the JSON body.
func stripNotice(b []byte) []byte {
	s := string(b)
	if i := strings.IndexByte(s, '{'); i > 0 {
		return []byte(s[i:])
	}
	return b
}
