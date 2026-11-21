package review

import (
	"fmt"
	"strings"
	"testing"
)

func addedFuncDiff(file, header string, startLine, logic, markup int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n--- /dev/null\n+++ b/%s\n@@ -0,0 +%d,%d @@\n", file, file, file, startLine, logic+markup+1)
	b.WriteString("+" + header + "\n")
	for i := 0; i < logic; i++ {
		fmt.Fprintf(&b, "+  const v%d = compute(%d);\n", i, i)
	}
	for i := 0; i < markup; i++ {
		if i%2 == 0 {
			b.WriteString("+      <div className=\"row\">\n")
		} else {
			b.WriteString("+      </div>\n")
		}
	}
	return b.String()
}

func largeFuncFindings(diff string) []Finding {
	ctx := FreshContext{Reviewer: "q", Diff: diff, Standards: []string{"quality"}}
	return reviewQualityLargeFuncs(&ctx)
}

func TestLargeFuncFindingCarriesLine(t *testing.T) {
	got := largeFuncFindings(addedFuncDiff("src/a.ts", "export function applyOps() {", 12, 90, 0))
	if len(got) != 1 || got[0].Line != 12 {
		t.Fatalf("want one finding at line 12, got %#v", got)
	}
}

func TestLargeFuncReactComponentDiscountsJSX(t *testing.T) {
	// 60 logic + 80 JSX lines: a normal component, not flagged.
	if got := largeFuncFindings(addedFuncDiff("apps/web/src/App.tsx", "export function App() {", 1, 60, 80)); len(got) != 0 {
		t.Fatalf("JSX-heavy component should not be flagged: %#v", got)
	}
	// 130 logic lines: still flagged, with the component message.
	got := largeFuncFindings(addedFuncDiff("apps/web/src/Canvas.tsx", "export function Canvas() {", 5, 130, 130))
	if len(got) != 1 || !strings.Contains(got[0].Message, "React component") || got[0].Line != 5 {
		t.Fatalf("large component should be flagged at line 5: %#v", got)
	}
	// Lowercase function in .tsx (hook/helper) keeps the regular limit.
	if got := largeFuncFindings(addedFuncDiff("apps/web/src/useSync.tsx", "export function useSync() {", 1, 85, 0)); len(got) != 1 {
		t.Fatalf("hook keeps regular limit: %#v", got)
	}
}

func TestToolchainCriterionCoveredByVerifyPass(t *testing.T) {
	crit := "build, typecheck, testes e E2E passam"
	ctx := FreshContext{
		Reviewer:  "a",
		Ticket:    &TicketScope{Acceptance: []string{crit}},
		Changed:   []string{"packages/core/src/ops.ts"},
		Diff:      "diff --git a/packages/core/src/ops.ts b/packages/core/src/ops.ts\n+export const x = 1\n",
		Standards: []string{"acceptance"},
	}
	if got := reviewAcceptance(&ctx); len(got) != 1 {
		t.Fatalf("without verify the criterion has no evidence: %#v", got)
	}
	ctx.VerifyVerdict = "PASS"
	if got := reviewAcceptance(&ctx); len(got) != 0 {
		t.Fatalf("verify PASS in the same stack should cover it: %#v", got)
	}
	ctx.VerifyVerdict = "ESCALATE"
	if got := reviewAcceptance(&ctx); len(got) != 1 {
		t.Fatalf("non-PASS verify is not evidence: %#v", got)
	}
}

func TestToolchainCriterionNeedsSubjectAndOutcome(t *testing.T) {
	for crit, want := range map[string]bool{
		"all tests pass":                                            true,
		"build and lint are clean":                                  true,
		"build, typecheck, testes e E2E passam":                     true,
		"login returns 200 on success":                              false,
		"tests cover the undo path":                                 false,
		"unit tests pass for refund endpoint, 404 on unknown order": false,
	} {
		if got := toolchainCriterion(crit); got != want {
			t.Errorf("%q: got %v want %v", crit, got, want)
		}
	}
}
