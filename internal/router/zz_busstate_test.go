package router

import (
	"os"
	"testing"
)

// Isolates the bus private dir (sessions) in a temp dir so tests never
// write to $HOME.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tb-router-bus-state-")
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
