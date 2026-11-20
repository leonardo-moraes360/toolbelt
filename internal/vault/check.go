package vault

import "sort"

// CheckResult is tb vault check (R16).
type CheckResult struct {
	OK       bool    `json:"ok"`
	Vault    string  `json:"vault"`
	Notes    int     `json:"notes"`
	Errors   int     `json:"errors"`
	Warnings int     `json:"warnings"`
	Issues   []Issue `json:"issues"`
	ExitCode int     `json:"exit_code"`
}

var linkIssue = map[string]string{
	StateBroken:            "broken_link",
	StateAmbiguous:         "ambiguous_link",
	StateBrokenFragment:    "broken_fragment",
	StateMissingAttachment: "missing_attachment",
}

var issueLevel = map[string]string{
	"broken_link": LevelError, "ambiguous_link": LevelError,
	"broken_fragment": LevelWarning, "missing_attachment": LevelWarning,
}

// Check validates the whole vault (R16): errors give exit 1, warnings
// never change the exit code.
func (v *Vault) Check() (*CheckResult, error) {
	sc, err := v.scan()
	if err != nil {
		return nil, err
	}
	ix := newIndex(sc)
	issues := append([]Issue{}, sc.issues...)
	for _, n := range sc.notes {
		issues = append(issues, n.Issues...)
		for _, l := range ix.links(n) {
			code, bad := linkIssue[l.State]
			if !bad {
				continue
			}
			issues = append(issues, Issue{Code: code, Level: issueLevel[code], File: n.File, Line: l.Line, Col: l.Col,
				Data: &IssueData{Target: linkText(l.Data)}})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Code < b.Code
	})
	res := &CheckResult{OK: true, Vault: v.Rel, Notes: len(sc.notes), Issues: redactIssues(issues)}
	if res.Issues == nil {
		res.Issues = []Issue{}
	}
	for _, is := range res.Issues {
		if is.Level == LevelError {
			res.Errors++
		} else {
			res.Warnings++
		}
	}
	if res.Errors > 0 {
		res.OK, res.ExitCode = false, ExitFail
	}
	return res, nil
}

func linkText(d LinkData) string {
	if d.Fragment == "" {
		return d.Target
	}
	return d.Target + "#" + d.Fragment
}
