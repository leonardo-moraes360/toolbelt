package lab

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tempRepo makes a repo root with a .toolbelt dir and isolates lab state via
// TB_LAB_STATE_DIR, returning the root.
func tempRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvStateDir, t.TempDir())
	return root
}

func writeManifest(t *testing.T, root, name, body string) {
	t.Helper()
	dir := Dir(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	root := tempRepo(t)
	if st, err := ReadState(root, "x"); err != nil || st != nil {
		t.Fatalf("ReadState absent = (%v, %v), want (nil, nil)", st, err)
	}
	s := &State{Name: "x", Tier: TierSandbox, ProvisionedAt: time.Now().UTC()}
	if err := WriteState(root, s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadState(root, "x")
	if err != nil || got == nil || got.Tier != TierSandbox {
		t.Fatalf("ReadState = (%+v, %v)", got, err)
	}
	prov, _ := Provisioned(root)
	if len(prov) != 1 || prov[0] != "x" {
		t.Errorf("Provisioned = %v, want [x]", prov)
	}
	// RemoveState is idempotent.
	if err := RemoveState(root, "x"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveState(root, "x"); err != nil {
		t.Errorf("second RemoveState should be nil, got %v", err)
	}
	if st, _ := ReadState(root, "x"); st != nil {
		t.Error("state should be gone after down")
	}
	if prov, _ := Provisioned(root); len(prov) != 0 {
		t.Errorf("Provisioned after down = %v, want empty", prov)
	}
}

func TestStateRejectsTraversalName(t *testing.T) {
	root := tempRepo(t)
	bad := "../../../../tmp/evil"
	if _, err := ReadState(root, bad); err == nil {
		t.Error("ReadState must reject a path-traversal name")
	}
	if err := RemoveState(root, bad); err == nil {
		t.Error("RemoveState must reject a path-traversal name")
	}
	if err := WriteState(root, &State{Name: bad, Tier: TierSandbox}); err == nil {
		t.Error("WriteState must reject a path-traversal name")
	}
}

func TestListAndLoad(t *testing.T) {
	root := tempRepo(t)
	writeManifest(t, root, "sqli", "name: sqli\nclass: sql-injection\ntier: sandbox\nnodes:\n  target:\n    build: b\n")
	writeManifest(t, root, "xss", "name: xss\nclass: xss\ntier: sandbox\nnodes:\n  target:\n    build: b\n")

	names, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "sqli" || names[1] != "xss" {
		t.Errorf("List = %v, want [sqli xss]", names)
	}
	l, err := Load(root, "sqli")
	if err != nil || l.Class != "sql-injection" {
		t.Fatalf("Load = (%+v, %v)", l, err)
	}
	if _, err := Load(root, "missing"); err == nil {
		t.Error("Load of missing lab should error")
	}
}

func TestLoadNameMismatch(t *testing.T) {
	root := tempRepo(t)
	// Manifest name disagrees with the directory name.
	writeManifest(t, root, "sqli", "name: other\nclass: sql-injection\ntier: sandbox\nnodes:\n  target:\n    build: b\n")
	if _, err := Load(root, "sqli"); err == nil {
		t.Error("Load should reject a name/dir mismatch")
	}
}
