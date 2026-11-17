//go:build !unix

package router

import (
	"errors"
	"fmt"
	"os"
)

var errRouterBusy = errors.New("outro tb router run ativo neste repositório")

// acquireLock (sem flock): lockfile O_EXCL removido ao sair. Se o processo
// morrer sem limpar, o arquivo fica e o usuário precisa removê-lo.
func acquireLock(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // .toolbelt/router/lock
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w (se não houver outro router, remova %s)", errRouterBusy, path)
		}
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}
