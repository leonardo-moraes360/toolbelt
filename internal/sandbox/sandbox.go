package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var ErrUnsupported = errors.New("Landlock indisponível nesta plataforma")

type Config struct {
	WorkDir          string
	AllowedPaths     []PathAccess
	ForbiddenPaths   []string
	Timeout          time.Duration
	AllowUnsandboxed bool
	// Env, quando não nil, é o ambiente completo do processo filho (use
	// execenv.Filter para remover segredos). nil mantém o comportamento
	// anterior: o filho herda o ambiente do processo atual.
	Env []string
	// ProcessGroup isola o filho num grupo de processos próprio e mata o grupo
	// inteiro ao cancelar o contexto/timeout (usado pelo router/watchdog).
	ProcessGroup bool
	// Registry, quando não nil (só com ProcessGroup), rastreia o pid do filho e,
	// após o agente terminar, espera o grupo esvaziar para que descendentes que
	// escaparam (setsid/setpgid) apareçam como órfãos em Registry.Sweep. nil
	// (padrão, `tb sandbox run`) mantém o comportamento anterior.
	Registry *Registry
	// GroupOwner identifica o dono do grupo (o ticket) no Registry: descendentes
	// órfãos achados nesse grupo cujo ambiente aponte outro dono são tratados como
	// escape (ver Registry.sweepLocked). Só vale com ProcessGroup e Registry.
	GroupOwner string

	// ReadOnlyWorkDir torna a regra do workdir somente leitura (padrão: leitura
	// e escrita).
	ReadOnlyWorkDir bool
	// PrivateTmp, quando não vazio, é um diretório gravável que substitui a
	// regra de escrita em os.TempDir() e é exportado como TMPDIR.
	PrivateTmp string
	// Home, quando não vazio, é um diretório gravável exportado como HOME e
	// como base de XDG_CONFIG_HOME, XDG_CACHE_HOME e XDG_DATA_HOME. Sobrescreve
	// os valores de Env.
	Home string
	// Path, quando não vazio, é exportado como PATH e sobrescreve Env.
	Path string
	// Stdin, quando não nil, é a entrada padrão do filho.
	Stdin io.Reader
	// Stdout, quando não nil, recebe só o stdout do filho (o stderr continua
	// em Result.Output). MaxStdoutBytes limita o que é repassado: o excedente é
	// lido e descartado (o filho não trava no pipe) e Result.StdoutOverflow
	// fica true. MaxStdoutBytes <= 0 usa MaxOutputBytes.
	Stdout         io.Writer
	MaxStdoutBytes int
	// MinimalDev troca a leitura de /dev inteiro por regras só para /dev/null,
	// /dev/zero, /dev/full, /dev/random e /dev/urandom (sem /dev/pts e
	// /dev/shm).
	MinimalDev bool
}

type PathAccess struct {
	Path     string
	ReadOnly bool
}

type Sandbox struct {
	config     Config
	landlockOK bool
}

type Result struct {
	Output    string        `json:"output"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration_ns"`
	TimedOut  bool          `json:"timed_out"`
	Canceled  bool          `json:"canceled,omitempty"`
	Sandboxed bool          `json:"sandboxed"`
	// StdoutOverflow indica que o stdout passou de MaxStdoutBytes (só com
	// Config.Stdout).
	StdoutOverflow bool `json:"stdout_overflow,omitempty"`
}

func New(config Config) (*Sandbox, error) { //nolint:gocritic // API pública estável por valor

	if config.WorkDir == "" {
		return nil, errors.New("workdir obrigatório")
	}
	abs, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("workdir: %w", err)
	}
	config.WorkDir = abs

	s := &Sandbox{config: config}
	if runtime.GOOS == "linux" {
		s.landlockOK = landlockAvailable()
	}
	if !s.landlockOK && !config.AllowUnsandboxed {
		if runtime.GOOS != "linux" {
			return nil, fmt.Errorf("%w; use --allow-unsandboxed para optar por execução sem isolamento", ErrUnsupported)
		}
		_, _, probeErr := landlockSupported()
		if probeErr == nil {
			probeErr = ErrUnsupported
		}
		return nil, fmt.Errorf("Landlock indisponível (%v); use --allow-unsandboxed para optar por execução sem isolamento", probeErr)
	}
	return s, nil
}

func (s *Sandbox) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	bin := command
	if !filepath.IsAbs(command) {
		var err error
		bin, err = exec.LookPath(command)
		if err != nil {
			return nil, fmt.Errorf("comando não encontrado: %w", err)
		}
	}

	if s.landlockOK {
		return s.runIsolated(ctx, bin, args...)
	}
	return s.runPlain(ctx, bin, args...)
}

func (s *Sandbox) runPlain(ctx context.Context, bin string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // sandbox invocado explicitamente pelo operador
	cmd.Dir = s.config.WorkDir
	if s.config.Env != nil || s.config.hasEnvOverrides() {
		base := s.config.Env
		if base == nil {
			base = os.Environ()
		}
		cmd.Env = s.config.applyEnvOverrides(base)
	}
	return finishRun(ctx, cmd, false, &s.config)
}

func (s *Sandbox) runIsolated(ctx context.Context, bin string, args ...string) (*Result, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("executável atual: %w", err)
	}
	env, err := prepareChildEnv(&s.config, bin, args)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, self) //nolint:gosec // re-exec do próprio binário como trampolim Landlock
	cmd.Dir = s.config.WorkDir
	cmd.Env = env
	return finishRun(ctx, cmd, true, &s.config)
}

func (c *Config) hasEnvOverrides() bool {
	return c.Home != "" || c.Path != "" || c.PrivateTmp != ""
}

// applyEnvOverrides devolve uma cópia de env com HOME/XDG_*, PATH e TMPDIR
// substituídos pelos campos Home, Path e PrivateTmp (os que não estão vazios).
func (c *Config) applyEnvOverrides(env []string) []string {
	set := map[string]string{}
	if c.Home != "" {
		set["HOME"] = c.Home
		set["XDG_CONFIG_HOME"] = filepath.Join(c.Home, ".config")
		set["XDG_CACHE_HOME"] = filepath.Join(c.Home, ".cache")
		set["XDG_DATA_HOME"] = filepath.Join(c.Home, ".local", "share")
	}
	if c.Path != "" {
		set["PATH"] = c.Path
	}
	if c.PrivateTmp != "" {
		set["TMPDIR"] = c.PrivateTmp
	}
	out := make([]string, 0, len(env)+len(set))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if _, ok := set[k]; ok {
			continue
		}
		out = append(out, e)
	}
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "PATH", "TMPDIR"} {
		if v, ok := set[k]; ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// limitedWriter repassa até max bytes para w e descarta o resto, sem devolver
// erro (o filho nunca trava no pipe).
type limitedWriter struct {
	mu       sync.Mutex
	w        io.Writer
	max      int
	written  int
	overflow bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(p)
	room := l.max - l.written
	if room <= 0 {
		l.overflow = true
		return n, nil
	}
	chunk := p
	if len(chunk) > room {
		chunk = chunk[:room]
		l.overflow = true
	}
	if _, err := l.w.Write(chunk); err != nil {
		l.overflow = true
	}
	l.written += len(chunk)
	return n, nil
}

func (l *limitedWriter) Overflow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.overflow
}

// MaxOutputBytes limita a saída do filho mantida em memória/log: só o final
// (tail) é preservado.
const MaxOutputBytes = 1 << 20

const truncatedMarker = "[... saída anterior truncada; mantidos os últimos 1 MiB ...]\n"

// tailBuffer é um io.Writer seguro entre goroutines que guarda só os últimos
// max bytes escritos. Nunca devolve erro (não aplica backpressure ao filho).
type tailBuffer struct {
	mu        sync.Mutex
	max       int
	buf       []byte
	truncated bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.max {
		n := copy(t.buf, t.buf[len(t.buf)-t.max:])
		t.buf = t.buf[:n]
		t.truncated = true
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	b := t.buf
	if len(b) > t.max {
		b = b[len(b)-t.max:]
		t.truncated = true
	}
	if t.truncated {
		return truncatedMarker + string(b)
	}
	return string(b)
}

func finishRun(ctx context.Context, cmd *exec.Cmd, sandboxed bool, cfg *Config) (*Result, error) {
	group, reg, owner := cfg.ProcessGroup, cfg.Registry, cfg.GroupOwner
	if group {
		killProcessGroup(cmd)
		// Pdeathsig (linux) vale para a thread que faz o fork: mantém esta
		// goroutine presa à mesma thread até o filho terminar.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	out := &tailBuffer{max: MaxOutputBytes}
	cmd.Stdout = out
	cmd.Stderr = out
	var stdout *limitedWriter
	if cfg.Stdout != nil {
		limit := cfg.MaxStdoutBytes
		if limit <= 0 {
			limit = MaxOutputBytes
		}
		stdout = &limitedWriter{w: cfg.Stdout, max: limit}
		cmd.Stdout = stdout
	}
	if cfg.Stdin != nil {
		cmd.Stdin = cfg.Stdin
	}
	start := time.Now()
	var err error
	if group && reg != nil {
		err = reg.RunOwned(cmd, true, owner)
	} else {
		err = cmd.Run()
	}
	if group {
		// Em qualquer caminho (sucesso, falha, cancelamento) nada do grupo pode
		// sobreviver ao agente: netos em segundo plano são encerrados.
		reapProcessGroup(cmd)
		if reg != nil && cmd.Process != nil {
			settleGroup(reg, cmd.Process.Pid)
		}
	}
	res := &Result{Output: out.String(), Duration: time.Since(start), Sandboxed: sandboxed}
	if stdout != nil {
		res.StdoutOverflow = stdout.Overflow()
	}

	// Agente saiu com 0 mas deixou um filho segurando stdout/stderr: o Go devolve
	// ErrWaitDelay. O agente terminou com sucesso; o grupo já foi encerrado acima.
	if err != nil && errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		err = nil
	}

	if err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			res.TimedOut = true
			res.ExitCode = -1
			return res, nil
		case context.Canceled:
			res.Canceled = true
			res.ExitCode = -1
			return res, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return nil, err
	}
	return res, nil
}

func (s *Sandbox) ValidatePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	work, err := filepath.Abs(s.config.WorkDir)
	if err != nil {
		return err
	}
	for _, forbidden := range s.config.ForbiddenPaths {
		base := forbidden
		if !filepath.IsAbs(forbidden) {
			base = filepath.Join(work, forbidden)
		}
		if abs == base || isSubpath(abs, base) {
			return fmt.Errorf("caminho proibido: %s", forbidden)
		}
	}
	if isSubpath(abs, work) || abs == work {
		return nil
	}
	return fmt.Errorf("fora do escopo: %s", path)
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && rel != "."
}

func landlockAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	ok, _, _ := landlockSupported()
	return ok
}

// landlockSupported é a sonda de Landlock usada por New. É uma variável para que
// os testes simulem um host sem Landlock (ver OverrideLandlockProbe); em
// produção é sempre CheckLandlockSupport.
var landlockSupported = CheckLandlockSupport

// OverrideLandlockProbe substitui a sonda de Landlock e devolve a função que a
// restaura. SOMENTE para testes (do próprio pacote e de pacotes que precisam
// exercitar o fail-closed, como o router): permite verificar que sem Landlock e
// sem AllowUnsandboxed nada é executado, independentemente do kernel do CI.
func OverrideLandlockProbe(fn func() (bool, int, error)) (restore func()) {
	prev := landlockSupported
	landlockSupported = fn
	return func() { landlockSupported = prev }
}
