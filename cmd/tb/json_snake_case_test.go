package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
	"github.com/leonardo-moraes360/toolbelt/internal/version"
	"github.com/leonardo-moraes360/toolbelt/internal/workspace"
)

func TestJSONOutputSnakeCaseKeys(t *testing.T) {
	samples := []any{
		sandbox.Result{
			Output:    "ok\n",
			ExitCode:  0,
			Duration:  time.Millisecond,
			TimedOut:  false,
			Sandboxed: true,
		},
		mem.Invariant{
			ID:        "inv-1",
			Category:  "anti_cheat",
			Rule:      "não apague testes",
			AddedAt:   time.Unix(0, 0).UTC(),
			Immutable: true,
		},
		mem.CheckReport{
			Path:        ".toolbelt/invariants.yaml",
			SHA256:      "abc",
			Count:       1,
			Retention:   1,
			CompactSafe: true,
		},
		&workspace.Workspace{
			ID:        "ws-1",
			Path:      "/tmp/ws",
			BaseRef:   "HEAD",
			Branch:    "tb-workspace/ws-1",
			CreatedAt: time.Unix(0, 0).UTC(),
			Scope: workspace.Scope{
				BaseCommit:     "abc",
				AllowedPaths:   []string{"cmd"},
				ForbiddenPaths: []string{"secrets"},
			},
		},
		guard.CheckResult{
			BaseRef: "HEAD~1",
			HeadRef: "HEAD",
			Violations: []guard.Violation{{
				Type:     guard.ViolationProtectedPath,
				File:     "tests/hidden/x_test.go",
				Message:  "protegido",
				Severity: guard.SeverityError,
				Source:   guard.SourceDefault,
			}},
		},
		&review.Report{
			Target:  "rd.md",
			BaseRef: "HEAD~1",
			HeadRef: "HEAD",
			Findings: []review.Finding{{
				ID:        "scope-missing",
				Severity:  review.SeverityWarning,
				Category:  "scope",
				Message:   "sem scope",
				Reviewer:  "goldfish-1",
				Agreement: 3,
			}},
			Drift: &review.DriftReport{
				BaseMismatch: false,
				ActualBase:   "HEAD~1",
			},
			FreshOK:   true,
			Reviewers: 3,
		},
		&verify.Result{
			Verdict:       verify.VerdictPass,
			Iterations:    1,
			BuildRan:      true,
			BuildPassed:   true,
			VisiblePassed: true,
		},
		version.Get(),
	}

	for i, sample := range samples {
		t.Run(fmt.Sprintf("sample_%d", i), func(t *testing.T) {
			assertNoPascalCaseJSONKeys(t, sample)
		})
	}
}

func assertNoPascalCaseJSONKeys(t *testing.T, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
	var bad []string
	collectPascalCaseKeys(decoded, "", &bad)
	if len(bad) > 0 {
		t.Fatalf("chaves PascalCase em saída JSON (use snake_case): %v\npayload: %s", bad, data)
	}
}

func collectPascalCaseKeys(v any, path string, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			if isPascalCaseKey(k) {
				*out = append(*out, childPath)
			}
			collectPascalCaseKeys(child, childPath, out)
		}
	case []any:
		for i, child := range x {
			collectPascalCaseKeys(child, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

func isPascalCaseKey(key string) bool {
	if key == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(key)
	return unicode.IsUpper(r)
}
