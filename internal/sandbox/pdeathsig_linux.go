//go:build linux

package sandbox

import "syscall"

// setPdeathsig pede ao kernel que mate o filho (SIGKILL) se o processo pai
// morrer. Atenção: Pdeathsig é atrelado à *thread* que fez o fork; por isso
// finishRun trava a goroutine na thread (runtime.LockOSThread) até o filho
// terminar. Cobre morte abrupta do router (kill -9); o grupo restante é
// limitado pelo próprio filho morto + reapProcessGroup nos caminhos normais.
func setPdeathsig(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGKILL }
