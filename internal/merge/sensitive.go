package merge

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/guard"
)

// ErrSensitive é devolvido (embrulhado) quando um ticket altera caminhos que
// podem virar execução de código fora do sandbox (hooks, config, CI...).
var ErrSensitive = errors.New("caminhos sensíveis alterados")

// hardenedGitArgs neutralizam execução de código via config/hooks: o merge
// roda git FORA do sandbox sobre conteúdo escrito por agentes.
var hardenedGitArgs = []string{
	"-c", "core.hooksPath=" + os.DevNull,
	"-c", "core.fsmonitor=false",
	"-c", "diff.external=",
	"-c", "core.pager=cat",
	"-c", "commit.gpgsign=false",
	"-c", "maintenance.auto=false",
	"-c", "gc.auto=0",
	"-c", "submodule.recurse=false",
}

// Sensitive descreve um caminho sensível alterado por um ticket.
type Sensitive struct {
	Ticket string `json:"ticket"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type change struct {
	path    string
	gitlink bool
}

// changes lista os caminhos alterados em base..head (sem renames) com modos.
func changes(r runner, base, head string) ([]change, error) {
	out, err := r.git(r.repo, "diff", "--raw", "--no-ext-diff", "--no-renames", "-z", "--no-abbrev", base, head)
	if err != nil {
		return nil, err
	}
	var res []change
	parts := strings.Split(out, "\x00")
	for i := 0; i+1 < len(parts); i += 2 {
		meta := strings.Fields(strings.TrimPrefix(parts[i], ":"))
		if len(meta) < 2 {
			continue
		}
		res = append(res, change{path: parts[i+1], gitlink: meta[0] == "160000" || meta[1] == "160000"})
	}
	return res, nil
}

// hooksDirs devolve os diretórios (relativos à raiz, com "/" final) de hooks.
func hooksDirs(r runner) []string {
	dirs := []string{".husky/", ".githooks/"}
	// -c core.hooksPath (command scope) esconderia o valor real: lê por escopo.
	hp := ""
	for _, scope := range []string{"--system", "--global", "--local"} {
		if v, _ := r.out(r.repo, "config", scope, "--get", "core.hooksPath"); strings.TrimSpace(v) != "" {
			hp = strings.TrimSpace(v)
		}
	}
	if hp == "" || hp == os.DevNull {
		return dirs
	}
	if filepath.IsAbs(hp) {
		rel, err := filepath.Rel(r.repo, hp)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return dirs
		}
		hp = rel
	}
	hp = path.Clean(filepath.ToSlash(hp))
	if hp == "." || strings.HasPrefix(hp, "../") || hp == ".." {
		return dirs
	}
	return append(dirs, hp+"/")
}

func sensitiveReason(p string, c change, hooks []string) string {
	lp := strings.ToLower(p)
	if c.gitlink {
		return "gitlink/submódulo"
	}
	for _, h := range hooks {
		if strings.HasPrefix(lp, strings.ToLower(h)) {
			return "diretório de hooks (" + strings.TrimSuffix(h, "/") + ")"
		}
	}
	switch {
	case strings.HasPrefix(lp, ".toolbelt/"):
		return "configuração do toolbelt"
	case strings.HasPrefix(lp, ".github/workflows/"):
		return "workflow de CI"
	case lp == ".gitmodules":
		return ".gitmodules"
	case path.Base(lp) == ".gitattributes":
		return ".gitattributes (filtros)"
	}
	return ""
}

// detectSensitive verifica, por ticket, os caminhos sensíveis alterados.
// Também reaproveita os paths protegidos do guard (defaults + guard.yaml da base).
func detectSensitive(r runner, prep []prepared) ([]Sensitive, error) {
	hooks := hooksDirs(r)
	out := []Sensitive{}
	for _, p := range prep {
		base, head := p.base, p.head
		cs, err := changes(r, base, head)
		if err != nil {
			return nil, err
		}
		reasons := map[string]string{}
		for _, c := range cs {
			if why := sensitiveReason(c.path, c, hooks); why != "" {
				reasons[c.path] = why
			}
		}
		if gr, gerr := guard.NewChecker(r.repo).Check(base, head); gerr == nil {
			for _, v := range gr.Violations {
				if v.Type == guard.ViolationProtectedPath {
					if _, ok := reasons[v.File]; !ok {
						reasons[v.File] = "path protegido (guard)"
					}
				}
			}
		}
		paths := make([]string, 0, len(reasons))
		for f := range reasons {
			paths = append(paths, f)
		}
		sort.Strings(paths)
		for _, f := range paths {
			out = append(out, Sensitive{Ticket: p.t.ID, Path: f, Reason: reasons[f]})
		}
	}
	return out, nil
}
