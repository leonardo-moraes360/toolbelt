package lab

import (
	"fmt"
	"regexp"

	"github.com/leonardo-moraes360/toolbelt/internal/lab/scenario"
	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
)

// Defense reports whether the blue side detected the attack.
type Defense string

const (
	// DefenseDetected means a blue check flagged the target's vulnerability.
	DefenseDetected Defense = "detected"
	// DefenseMissed means no blue check flagged it.
	DefenseMissed Defense = "missed"
)

// Verdict is the end-to-end result of a sandbox-tier lab run: did the attack
// reproduce, and did the paired blue detection catch it.
type Verdict struct {
	Lab          string          `json:"lab"`
	Class        string          `json:"class"`
	Tier         Tier            `json:"tier"`
	Attack       scenario.Attack `json:"attack"`
	Defense      Defense         `json:"defense"`
	MatchedCheck string          `json:"matched_check,omitempty"`
	Detail       string          `json:"detail"`
	Observable   string          `json:"observable"`
}

// RunSandbox drives a lab's class scenario in the sandbox tier and reports the
// attack/defense verdict. The scenario is pure-Go and in-process (it opens no
// sockets and reads no host env), so the sandbox contract here is the
// fail-closed tier gate (Landlock on Linux) plus the scenario's by-construction
// isolation; Phase 2's Podman tier wraps real external targets. The blue side
// reuses the class's registered checks (secplay), so a lab proves the same
// detector that `tb blue check` would run.
func RunSandbox(l *Lab) (*Verdict, error) {
	avail := ProbeSandbox()
	if !avail.Available {
		return nil, fmt.Errorf("sandbox tier unavailable: %s", avail.Remediation)
	}

	sc, err := scenario.For(l.Class)
	if err != nil {
		return nil, err
	}

	corpus, err := secplay.Load()
	if err != nil {
		return nil, err
	}
	checks, err := corpus.ChecksForClass(l.Class)
	if err != nil {
		return nil, fmt.Errorf("lab %q: %w", l.Name, err)
	}

	out := sc.Run()

	v := &Verdict{
		Lab:        l.Name,
		Class:      l.Class,
		Tier:       TierSandbox,
		Attack:     out.Attack,
		Defense:    DefenseMissed,
		Detail:     out.Detail,
		Observable: out.Observable,
	}
	for _, c := range checks {
		re, err := regexp.Compile(c.Pattern)
		if err != nil {
			return nil, fmt.Errorf("blue check %q: bad pattern: %w", c.ID, err)
		}
		if re.MatchString(out.Observable) {
			v.Defense = DefenseDetected
			v.MatchedCheck = c.ID
			break
		}
	}
	return v, nil
}
