package registry

import (
	"os"
	"testing"
)

// TestMain isolates the bus private directory (sessions, wake, harness
// state) in a temp directory: tests never write to $HOME.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tb-bus-state-")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("TB_BUS_STATE_DIR", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
