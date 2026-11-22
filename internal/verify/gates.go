package verify

import "context"

// CheckResult is one named check (R3) in the tests ruler's JSON.
type CheckResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// gate is a step that runs once per verify run and is cached across
// iterations; its failure is deterministic, so verify ends with ESCALATE.
type gate struct {
	// label prefixes the failure reason ("typecheck failed: …").
	label string
	// key names the config key in a missing-command error.
	key    string
	cmd    string
	done   bool
	ok     bool
	record func(res *Result, ok bool, reason string)
}

// sequentialGates run in order and stop at the first failure: build,
// typecheck, lint. Build and lint keep their config key as label so their
// reasons stay byte-identical to the pre-ruler output.
func sequentialGates(cfg *Config) []*gate {
	var gates []*gate
	if cfg.BuildCmd != "" {
		gates = append(gates, &gate{label: "verify.build_cmd", key: "verify.build_cmd", cmd: cfg.BuildCmd,
			record: func(res *Result, ok bool, _ string) { res.BuildRan, res.BuildPassed = true, ok }})
	}
	if cfg.TypecheckCmd != "" {
		gates = append(gates, &gate{label: "typecheck", key: "verify.typecheck_cmd", cmd: cfg.TypecheckCmd,
			record: func(res *Result, ok bool, _ string) { res.TypecheckRan, res.TypecheckPassed = true, boolPtr(ok) }})
	}
	if cfg.LintCmd != "" {
		gates = append(gates, &gate{label: "verify.lint_cmd", key: "verify.lint_cmd", cmd: cfg.LintCmd,
			record: func(res *Result, ok bool, _ string) { res.LintRan, res.LintPassed = true, ok }})
	}
	return gates
}

// checkGates all run, even after one fails, so the report lists every check.
func checkGates(cfg *Config) []*gate {
	gates := make([]*gate, 0, len(cfg.Checks))
	for _, c := range cfg.Checks {
		name := c.Name
		gates = append(gates, &gate{label: "check " + name, key: "verify.checks." + name, cmd: c.Cmd,
			record: func(res *Result, ok bool, reason string) {
				res.Checks = append(res.Checks, CheckResult{Name: name, Passed: ok, Reason: reason})
			}})
	}
	return gates
}

func runGates(ctx context.Context, cfg *Config, res *Result, cache *cmdCache) (bool, error) {
	for _, g := range cache.gates {
		ok, err := runGate(ctx, cfg, res, g)
		if err != nil || !ok {
			return false, err
		}
	}
	allOK := true
	failReasonText := ""
	for _, g := range cache.checks {
		ok, err := runGate(ctx, cfg, res, g)
		if err != nil {
			return false, err
		}
		if !ok && allOK {
			allOK = false
			failReasonText = res.Reason
		}
	}
	if !allOK {
		setFailure(res, "verify.checks", failReasonText)
	}
	return allOK, nil
}

// runGate records the step only on its first run: res persists across
// iterations, so a cached gate already shows its outcome.
func runGate(ctx context.Context, cfg *Config, res *Result, g *gate) (bool, error) {
	if g.done {
		return g.ok, nil
	}
	ok, out, runErr := runConfiguredCmd(ctx, cfg, g.cmd, g.key)
	g.done = true
	g.ok = ok && runErr == nil
	reason := ""
	switch {
	case runErr != nil:
		reason = runErr.Error()
	case !ok:
		reason = failReason(g.label, out)
	}
	g.record(res, g.ok, reason)
	if reason != "" {
		setFailure(res, g.key, reason)
	}
	return g.ok, runErr
}
