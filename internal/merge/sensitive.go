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

// ErrSensitive is returned (wrapped) when a ticket changes paths that can lead
// to code execution outside the sandbox (hooks, config, CI...).
var ErrSensitive = errors.New("sensitive paths changed")

// hardenedGitArgs neutralize code execution via config/hooks: merge runs git
// OUTSIDE the sandbox over agent-written content.
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

type Sensitive struct {
	Ticket string `json:"ticket"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type change struct {
	path    string
	gitlink bool
}

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

// hooksDirs returns hook directories relative to the repo root, with a trailing "/".
func hooksDirs(r runner) []string {
	dirs := []string{".husky/", ".githooks/"}
	// -c core.hooksPath (command scope) would hide the real value: read per scope.
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
		return "gitlink/submodule"
	}
	for _, h := range hooks {
		if strings.HasPrefix(lp, strings.ToLower(h)) {
			return "hooks directory (" + strings.TrimSuffix(h, "/") + ")"
		}
	}
	switch {
	case strings.HasPrefix(lp, ".toolbelt/"):
		return "toolbelt configuration"
	case strings.HasPrefix(lp, ".github/workflows/"):
		return "CI workflow"
	case lp == ".gitmodules":
		return ".gitmodules"
	case path.Base(lp) == ".gitattributes":
		return ".gitattributes (filtros)"
	}
	return ""
}

// detectSensitive also reuses guard's protected paths (defaults + the base's
// guard.yaml).
func detectSensitive(r runner, prep []prepared) ([]Sensitive, error) {
	hooks := hooksDirs(r)
	out := []Sensitive{}
	for i := range prep {
		p := &prep[i]
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
						reasons[v.File] = "protected path (guard)"
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
