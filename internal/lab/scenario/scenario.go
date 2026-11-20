// Package scenario holds the in-process, pure-Go seed targets a sandbox-tier
// lab exercises. A scenario runs a vulnerable target against a red technique
// (the attack) and exposes an observable — the construct a static blue check
// inspects (the defense). Scenarios open no sockets and import no net package,
// so a sandbox-tier lab has nothing to egress by construction.
package scenario

import (
	"fmt"
	"sort"
)

// Attack reports whether the red technique reproduced the vulnerability.
type Attack string

const (
	// AttackReproduced means the technique exploited the target.
	AttackReproduced Attack = "reproduced"
	// AttackFailed means the target resisted the technique.
	AttackFailed Attack = "failed"
)

// Outcome is a scenario's result: the attack verdict and the observable the
// blue side inspects for detection.
type Outcome struct {
	Attack     Attack `json:"attack"`
	Detail     string `json:"detail"`
	Observable string `json:"observable"`
}

// Scenario is one class's in-process red exercise.
type Scenario interface {
	// Class is the playbook class this scenario ties to (e.g. "sql-injection").
	Class() string
	// Run drives the vulnerable target against the red technique. It must not
	// open sockets or touch the host beyond CPU and memory.
	Run() Outcome
}

var registry = map[string]Scenario{}

// register adds a scenario, panicking on a duplicate class (programmer error).
func register(s Scenario) {
	if _, dup := registry[s.Class()]; dup {
		panic("scenario: duplicate class " + s.Class())
	}
	registry[s.Class()] = s
}

// For returns the scenario for a class, or an error naming the available ones.
func For(class string) (Scenario, error) {
	s, ok := registry[class]
	if !ok {
		return nil, fmt.Errorf("no lab scenario for class %q (have: %v)", class, Classes())
	}
	return s, nil
}

// Classes lists the registered scenario classes, sorted.
func Classes() []string {
	out := make([]string, 0, len(registry))
	for c := range registry {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
