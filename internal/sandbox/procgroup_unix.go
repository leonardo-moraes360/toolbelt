//go:build !windows

package sandbox

import (
	"os/exec"
	"syscall"
	"time"
)

// killProcessGroup faz o filho liderar seu próprio grupo de processos e, ao
// cancelar o contexto (timeout, watchdog, SIGINT), mata o grupo inteiro — assim
// netos que seguram o pipe de saída não travam Run.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(cmd.SysProcAttr)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}

// reapProcessGroup envia SIGKILL ao grupo do filho após Wait. ESRCH (grupo já
// vazio) é o caso normal e ignorado. O pgid é o pid do líder (Setpgid); a
// janela de reuso de pid entre o Wait e este kill é desprezível porque o grupo
// só some quando todos os membros morrem.
func reapProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// groupGone informa se não resta nenhum processo (nem zumbi) no grupo pgid.
func groupGone(pgid int) bool {
	return syscall.Kill(-pgid, 0) == syscall.ESRCH
}

// settleGroup espera (até ~1s) o grupo do agente esvaziar, colhendo os zumbis
// reparentados ao router (subreaper). Descendentes que escaparam do grupo
// ficam no registro de escapes e são entregues por Registry.Sweep. Ao esvaziar,
// a entrada do grupo é removida do registro (ver pruneGroupsLocked para o caso
// do grupo que não esvaziou a tempo).
func settleGroup(reg *Registry, pgid int) {
	reg.finishGroup(pgid)
	deadline := time.Now().Add(time.Second)
	for {
		reg.mu.Lock()
		reg.sweepLocked()
		reg.pruneGroupsLocked() // grupo vazio após o sweep: reap final, esquece a entrada
		reg.mu.Unlock()
		if groupGone(pgid) || time.Now().After(deadline) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}
