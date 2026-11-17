package sandbox

import (
	"os/exec"
	"sync"
)

// Registry rastreia os processos-filho que ESTE processo iniciou (líderes de
// agente e subprocessos git do router) e, no Linux com PR_SET_CHILD_SUBREAPER
// ativo (AcquireSubreaper), identifica e mata descendentes que escaparam do
// grupo de processos (setsid/setpgid) e foram reparentados ao router.
//
// Política: agentes não devem se desacoplar; qualquer processo-filho direto
// não rastreado é encerrado (SIGKILL) e colhido (wait4). Em plataformas sem
// subreaper (macOS/Windows) Sweep é no-op: só vale o comportamento de grupo de
// processos (limitação documentada).
//
// O mutex é mantido só durante cmd.Start+registro e durante Sweep: assim um
// filho recém-iniciado nunca é visto como órfão e o wait4 do Sweep nunca
// compete com o Wait do os/exec (só pids não rastreados são colhidos). Quem
// inicia processos por fora (ex.: bibliotecas que chamam exec direto) deve
// roteá-los por Run para ficarem rastreados; não existe lock mantido durante a
// vida do filho, então Sweep nunca fica bloqueado por um git lento.
type Registry struct {
	mu       sync.Mutex
	live     map[int]struct{}        // filhos diretos rastreados (entre Start e Wait)
	groups   map[groupKey]*groupInfo // grupos de agente, chaveados por (pid, starttime)
	found    []Orphan                // escapes detectados, ainda não entregues
	vars     []string                // variáveis de ambiente a capturar de cada escapado
	ownerVar string                  // variável que identifica o dono (ticket) de um grupo
	windows  []*spawnWindow          // janelas de spawn de processos não-agente (ex.: git worktree add)
}

// groupKey identifica um grupo de agente: o pid do líder sozinho pode ser
// reutilizado pelo kernel depois que o grupo some; o starttime desambigua.
type groupKey struct {
	pid   int
	start uint64
}

type groupInfo struct {
	owner    string // valor esperado de ownerVar nos membros ("" = não verificar)
	finished bool   // o agente líder terminou e o grupo já foi morto
}

type spawnWindow struct {
	from, to uint64 // ticks desde o boot (mesma base de starttime)
	open     bool
}

// Orphan descreve um descendente escapado encerrado por Sweep.
type Orphan struct {
	PID  int
	PGID int
	// Start é o starttime do processo (ticks desde o boot; 0 se desconhecido).
	Start uint64
	// Vars guarda só as variáveis de ambiente pedidas em NewRegistry
	// (atribuição ao ticket); nunca o ambiente inteiro (pode conter segredos).
	// nil = ambiente ilegível.
	Vars map[string]string
	// ForeignGroup: o processo foi achado dentro do grupo de OUTRO agente
	// (setpgid para o pgid alheio, para se esconder), com dono diferente do
	// grupo; GroupOwner é o dono do grupo.
	ForeignGroup bool
	GroupOwner   string
	// InSpawnWindow: nasceu dentro de uma janela de spawn (BeginSpawnWindow),
	// ou seja, pode ser um daemon de um git hook/fsmonitor do usuário e não de um
	// agente.
	InSpawnWindow bool
}

// NewRegistry cria um registro vazio. vars lista variáveis de ambiente a
// capturar de cada processo escapado antes de matá-lo (atribuição ao ticket).
func NewRegistry(vars ...string) *Registry {
	return &Registry{live: map[int]struct{}{}, groups: map[groupKey]*groupInfo{}, vars: vars}
}

// SetOwnerVar define qual variável (uma das passadas a NewRegistry) identifica o
// dono de cada grupo de agente iniciado com RunOwned. Um descendente órfão
// achado num grupo de agente cujo valor difere do dono do grupo é tratado como
// escape (ver sweepLocked).
func (r *Registry) SetOwnerVar(name string) {
	r.mu.Lock()
	r.ownerVar = name
	r.mu.Unlock()
}

// BeginSpawnWindow abre uma janela de tempo em que o chamador executa código de
// terceiros (ex.: `git worktree add`, que pode disparar hooks/fsmonitor do
// usuário) cujos daemons destacados não são culpa de nenhum agente. Descendentes
// escapados que nasceram dentro de uma janela vêm marcados em
// Orphan.InSpawnWindow. A função devolvida fecha a janela.
func (r *Registry) BeginSpawnWindow() (end func()) {
	w := &spawnWindow{from: sat(nowTicks(), 2), open: true}
	r.mu.Lock()
	r.windows = append(r.windows, w)
	if len(r.windows) > 64 {
		r.windows = append([]*spawnWindow(nil), r.windows[len(r.windows)-64:]...)
	}
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			t := nowTicks() + 2 // margem: starttime tem resolução de 10 ms
			r.mu.Lock()
			w.to, w.open = t, false
			r.mu.Unlock()
		})
	}
}

// sat subtrai d de v sem underflow.
func sat(v, d uint64) uint64 {
	if v < d {
		return 0
	}
	return v - d
}

func (r *Registry) inWindowLocked(start uint64) bool {
	if start == 0 {
		return false
	}
	for _, w := range r.windows {
		if start >= w.from && (w.open || start <= w.to) {
			return true
		}
	}
	return false
}

// Run inicia cmd (registrando o pid sob o mutex), espera e remove o registro.
// group indica que o filho lidera o próprio grupo (Setpgid).
func (r *Registry) Run(cmd *exec.Cmd, group bool) error {
	return r.RunOwned(cmd, group, "")
}

// RunOwned é Run com dono: owner é o valor esperado de ownerVar (ticket) nos
// processos do grupo; só tem efeito com group=true.
func (r *Registry) RunOwned(cmd *exec.Cmd, group bool, owner string) error {
	if err := r.start(cmd, group, owner); err != nil {
		return err
	}
	err := cmd.Wait()
	r.exited(cmd)
	return err
}

func (r *Registry) start(cmd *exec.Cmd, group bool, owner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	r.live[pid] = struct{}{}
	if group {
		r.groups[groupKey{pid, readStartTime(pid)}] = &groupInfo{owner: owner}
	}
	return nil
}

func (r *Registry) exited(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	r.mu.Lock()
	delete(r.live, cmd.Process.Pid)
	r.mu.Unlock()
}

// findGroupLocked acha o grupo de agente com líder pgid (o mais novo, se houver
// resíduo de um líder anterior com o mesmo pid).
func (r *Registry) findGroupLocked(pgid int) *groupInfo {
	var best *groupInfo
	var bestStart uint64
	for k, g := range r.groups {
		if k.pid == pgid && (best == nil || k.start >= bestStart) {
			best, bestStart = g, k.start
		}
	}
	return best
}

// finishGroup marca o grupo do agente como encerrado: órfãos nele passam a ser
// colhidos silenciosamente (já foram mortos por reapProcessGroup).
func (r *Registry) finishGroup(pgid int) {
	r.mu.Lock()
	if g := r.findGroupLocked(pgid); g != nil {
		g.finished = true
	}
	r.mu.Unlock()
}

// pruneGroupsLocked remove grupos encerrados cujo grupo de processos já sumiu.
// O kernel não reutiliza um pid enquanto existir grupo com esse pgid, então
// "grupo sumiu" é o momento seguro de esquecer a entrada (sem acúmulo nem
// colisão por reuso de pid).
func (r *Registry) pruneGroupsLocked() {
	for k, g := range r.groups {
		if g.finished && groupGone(k.pid) {
			delete(r.groups, k)
		}
	}
}

// Sweep mata e colhe todo filho direto não rastreado e devolve os escapes
// (inclusive os já encontrados por varreduras internas desde a última chamada).
func (r *Registry) Sweep() []Orphan {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	out := r.found
	r.found = nil
	return out
}

// sweepLocked classifica cada filho direto não rastreado:
//
//   - fora de qualquer grupo de agente: escape (setsid/setpgid para fora).
//   - em grupo de agente: um descendente de agente cujo pai morreu é reparentado
//     ao router MESMO sem escapar (ex.: `(sleep 30 &)`), então estar num grupo
//     de agente vivo, por si só, é normal e é deixado ao grupo. Mas um processo
//     só chega a um grupo "vivo" de ticket T por herança (é descendente de T) ou
//     por setpgid explícito para o pgid alheio, truque para se esconder da
//     varredura. Por isso comparamos a variável de dono (TB_TICKET_ID) do
//     processo com o dono do grupo: divergente, ausente ou ilegível (com o
//     processo ainda vivo) é tratado como escape, atribuído ao dono declarado no
//     próprio ambiente do processo (ou não atribuível, quando ausente). Zumbis
//     (nada a esconder, ambiente inacessível) são só colhidos.
func (r *Registry) sweepLocked() {
	r.pruneGroupsLocked()
	for _, c := range listChildren() {
		if _, tracked := r.live[c.pid]; tracked {
			continue
		}
		g := r.findGroupLocked(c.pgid)
		o := Orphan{PID: c.pid, PGID: c.pgid, Start: c.start}
		// Zumbi já morreu e não tem ambiente: nada a atribuir nem a matar, só
		// colher (um descendente vivo dele aparece como órfão próprio).
		escape := g == nil && !c.zombie
		if g == nil {
			o.Vars, _ = readEnvVars(c.pid, r.vars)
		} else if !c.zombie && g.owner != "" && r.ownerVar != "" {
			vars, ok := readEnvVars(c.pid, r.vars)
			foreign := false
			switch {
			case ok:
				foreign = vars[r.ownerVar] != g.owner
			default:
				foreign = procLive(c.pid) // ilegível mas vivo: não dá para provar o dono
			}
			if foreign {
				escape = true
				o.Vars, o.ForeignGroup, o.GroupOwner = vars, true, g.owner
			}
		}
		if !escape && g != nil && !g.finished {
			continue // descendente órfão de agente ainda rodando: o grupo o mata no fim
		}
		o.InSpawnWindow = r.inWindowLocked(c.start)
		killAndReap(c)
		if escape {
			r.found = append(r.found, o)
		}
	}
}
