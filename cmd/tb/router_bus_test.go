package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC28: router.bus in the repository's config is ignored; only the global
// config (or -c) counts.
func TestRouterBusOnlyFromTrustedConfig(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	repoCfg := "router:\n  max_workers: 0\n  bus:\n    channel: evil\n    as: router\n"
	if err := os.WriteFile(filepath.Join(root, ".toolbelt", "config.yaml"), []byte(repoCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	rc, _, err := loadRouterConfig(root)
	if err == nil || !strings.Contains(err.Error(), "max_workers") || rc.Bus.Channel != "" {
		t.Fatalf("repository router.bus was honored: %+v %v", rc.Bus, err)
	}
	global := filepath.Join(home, ".config", "toolbelt")
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "config.yaml"), []byte("router:\n  bus:\n    channel: good\n    as: router\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc, _, err = loadRouterConfig(root)
	if err != nil || rc.Bus.Channel != "good" || rc.Bus.Slots != 1 {
		t.Fatalf("global router.bus: %+v %v", rc.Bus, err)
	}
}
