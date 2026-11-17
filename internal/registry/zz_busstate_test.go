package registry

import (
	"os"
	"testing"
)

// TestMain isola o diretório privado do bus (sessões, wake, estado do
// harness) num diretório temporário: os testes nunca escrevem no $HOME.
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
