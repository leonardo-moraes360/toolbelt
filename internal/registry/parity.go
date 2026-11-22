package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
	"github.com/leonardo-moraes360/toolbelt/internal/stack"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

func init() {
	RegisterParityTools()
}

func RegisterParityTools() {
	registerMemTools()
	registerVerifyTools()
	registerReviewTools()
	registerGuardTools()
	registerStackTools()
	registerEgmTools()
	registerVaultTools()
}

func registerMemTools() {
	registerMemMutateTools()
	registerMemQueryTools()
}

func registerMemMutateTools() {
	Register(&Tool{
		Name:        "tb_mem_add",
		Description: "Adds an invariant to .toolbelt/invariants.yaml (parity with tb mem add)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"rule":      {Type: "string", Description: "Invariant text"},
				"category":  {Type: "string", Description: "anti_cheat|mandate|security", Default: "anti_cheat"},
				"ref":       {Type: "string", Description: "ADR/decisions.md path"},
				"immutable": {Type: "boolean", Description: "true to mark immutable (default mutable)", Default: false},
				"cwd":       {Type: "string", Description: "Repository directory"},
			},
			Required: []string{"rule"},
		},
		Handler: handleMemAdd,
	})
	Register(&Tool{
		Name:        "tb_mem_edit",
		Description: "Edits fields of an invariant (parity with tb mem edit)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":       {Type: "string", Description: "Invariant ID"},
				"category": {Type: "string", Description: "New category"},
				"rule":     {Type: "string", Description: "New rule"},
				"ref":      {Type: "string", Description: "New ADR/decisions.md path"},
				"force":    {Type: "boolean", Description: "true to edit immutable/anti_cheat", Default: false},
				"reason":   {Type: "string", Description: "Reason (required with force)"},
				"cwd":      {Type: "string", Description: "Repository directory"},
			},
			Required: []string{"id"},
		},
		Handler: handleMemEdit,
	})
	Register(&Tool{
		Name:        "tb_mem_retire",
		Description: "Retires an invariant (soft-retire; parity with tb mem retire)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":     {Type: "string", Description: "Invariant ID"},
				"force":  {Type: "boolean", Description: "true to retire immutable/anti_cheat", Default: false},
				"reason": {Type: "string", Description: "Reason (required with force)"},
				"cwd":    {Type: "string", Description: "Repository directory"},
			},
			Required: []string{"id"},
		},
		Handler: handleMemRetire,
	})
	Register(&Tool{
		Name:        "tb_mem_init",
		Description: "Creates the default anti-cheat invariants (parity with tb mem init)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd": {Type: "string", Description: "Repository directory"},
			},
		},
		Handler: handleMemInit,
	})
}

func registerMemQueryTools() {
	Register(&Tool{
		Name:        "tb_mem_list",
		Description: "Lists invariants (parity with tb mem list)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"category": {Type: "string", Description: "Filter by category"},
				"all":      {Type: "boolean", Description: "true to include retired", Default: false},
				"cwd":      {Type: "string", Description: "Repository directory"},
			},
		},
		Handler: handleMemList,
	})
	Register(&Tool{
		Name:        "tb_mem_show",
		Description: "Shows an invariant by id (parity with tb mem show)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":  {Type: "string", Description: "Invariant ID"},
				"cwd": {Type: "string", Description: "Repository directory"},
			},
			Required: []string{"id"},
		},
		Handler: handleMemShow,
	})
	Register(&Tool{
		Name:        "tb_mem_search",
		Description: "Searches invariants (parity with tb mem search)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Text to search in id/category/rule/ref"},
				"all":   {Type: "boolean", Description: "true to include retired", Default: false},
				"cwd":   {Type: "string", Description: "Repository directory"},
			},
			Required: []string{"query"},
		},
		Handler: handleMemSearch,
	})
	Register(&Tool{
		Name:        "tb_mem_check",
		Description: "Checks invariant integrity (parity with tb mem check)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"expect_sha": {Type: "string", Description: "Expected SHA256"},
				"all":        {Type: "boolean", Description: "true to include retired", Default: false},
				"cwd":        {Type: "string", Description: "Repository directory"},
			},
		},
		Handler: handleMemCheck,
	})
}

// checksProperty and skipChecksProperty are shared by tb_verify_run and
// tb_stack (R3, R5).
var (
	checksProperty = Property{
		Type:        "array",
		Description: "Named checks added or overridden: objects {name, cmd}; name matches [a-z0-9-]{1,32}",
		Items:       &Property{Type: "object"},
	}
	skipChecksProperty = Property{
		Type:        "array",
		Description: "Names of verify.checks entries to skip",
		Items:       &Property{Type: "string"},
	}
)

func registerVerifyTools() {
	Register(&Tool{
		Name:        "tb_verify_run",
		Description: "Runs the verification loop (parity with tb verify run)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":              {Type: "string", Description: "Repository directory"},
				"base":             {Type: "string", Description: "Ref base"},
				"head":             {Type: "string", Description: "Ref head"},
				"test_cmd":         {Type: "string", Description: "Visible tests command line"},
				"hidden_test_cmd":  {Type: "string", Description: "Hidden tests command line"},
				"build_cmd":        {Type: "string", Description: "Build command (empty to skip)"},
				"lint_cmd":         {Type: "string", Description: "Lint command (empty to skip)"},
				"no_build":         {Type: "boolean", Description: "Skips build_cmd", Default: false},
				"no_lint":          {Type: "boolean", Description: "Skips lint_cmd", Default: false},
				"dry_run":          {Type: "boolean", Description: "Shows the plan without running", Default: false},
				"max_iter":         {Type: "integer", Description: "Maximum iterations", Default: 0},
				"timeout":          {Type: "string", Description: "Timeout", Default: "10m"},
				"ruler":            {Type: "string", Description: "Rulers, comma-separated: tests|visual (visual runs alone; an unknown name is an error)", Default: "tests"},
				"typecheck_cmd":    {Type: "string", Description: "Typecheck command (empty to skip)"},
				"no_typecheck":     {Type: "boolean", Description: "Skips typecheck_cmd", Default: false},
				"checks":           checksProperty,
				"skip_checks":      skipChecksProperty,
				"no_checks":        {Type: "boolean", Description: "Skips every verify.checks entry", Default: false},
				"manifest":         {Type: "string", Description: "Visual manifest"},
				"threshold":        {Type: "number", Description: "Threshold visual", Default: 0.95},
				"skip_unavailable": {Type: "boolean", Description: "Graceful skip if ruler is unavailable", Default: true},
				"env": {
					Type:        "array",
					Description: "Extra keys for the environment allowlist",
					Items:       &Property{Type: "string"},
				},
				"pass_env": {
					Type:        "array",
					Description: passEnvArgDescription,
					Items:       &Property{Type: "string"},
				},
			},
		},
		Handler: handleVerifyRun,
	})
}

func registerReviewTools() {
	Register(&Tool{
		Name:        "tb_review",
		Description: "Fresh-context review against ticket (parity with tb review)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":                  {Type: "string", Description: "Repository directory"},
				"ticket":               {Type: "string", Description: "Path to ticket/rd.md", Default: "rd.md"},
				"base":                 {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":                 {Type: "string", Description: "Ref head", Default: "HEAD"},
				"approved_sha":         {Type: "string", Description: "Approved SHA; compares verbatim patch-id (whitespace-sensitive) with head"},
				"approved_base":        {Type: "string", Description: "Base of the approved range; default: merge-base(approved, base)"},
				"require_approved_sha": {Type: "boolean", Description: "Fails if approved_sha is missing", Default: false},
				"reviewers":            {Type: "integer", Description: "Maximum distinct goldfish", Default: 3},
				"standards": {
					Type:        "array",
					Description: "Roles: scope,quality,acceptance (accepts legacy CSV)",
					Default:     []string{"scope", "quality", "acceptance"},
					Items:       &Property{Type: "string"},
				},
			},
		},
		Handler: handleReview,
	})
	Register(&Tool{
		Name:        "tb_drift",
		Description: "Detects scope/base drift (parity with tb drift)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":                  {Type: "string", Description: "Repository directory"},
				"ticket":               {Type: "string", Description: "Path to ticket/rd.md", Default: "rd.md"},
				"base":                 {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":                 {Type: "string", Description: "Ref head", Default: "HEAD"},
				"approved_sha":         {Type: "string", Description: "Approved SHA; compares verbatim patch-id (whitespace-sensitive) with head"},
				"approved_base":        {Type: "string", Description: "Base of the approved range; default: merge-base(approved, base)"},
				"require_approved_sha": {Type: "boolean", Description: "Fails if approved_sha is missing", Default: false},
			},
		},
		Handler: handleDrift,
	})
}

func registerGuardTools() {
	Register(&Tool{
		Name:        "tb_guard_check",
		Description: "Analyzes the diff for anti-cheat violations (parity with tb guard check)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":       {Type: "string", Description: "Repository directory"},
				"base":      {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":      {Type: "string", Description: "Ref head", Default: "HEAD"},
				"range":     {Type: "string", Description: "base..head range (alternative to base/head)"},
				"warn_only": {Type: "boolean", Description: "true to not set strict_fail on SeverityError (opt-out)", Default: false},
				"strict":    {Type: "boolean", Description: "deprecated: strict_fail is already the default on SeverityError; use warn_only=true to opt out", Default: false},
			},
		},
		Handler: handleGuardCheck,
	})
}

func registerStackTools() {
	Register(&Tool{
		Name:        "tb_stack",
		Description: "Stacks verify/review/drift/guard/egm with shared context (parity with tb stack)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ops": {
					Type:        "array",
					Description: "Operations in order: verify, review, drift, guard, vault, egm (egm is not parallel and is always moved to the end)",
					Items:       &Property{Type: "string"},
				},
				"cwd":                  {Type: "string", Description: "Repository directory"},
				"base":                 {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":                 {Type: "string", Description: "Ref head", Default: "HEAD"},
				"fail_fast":            {Type: "boolean", Description: "Cancels the remaining steps on the first failure", Default: false},
				"jobs":                 {Type: "integer", Description: "Concurrency limit (0=number of CPUs, max 32; 1=serial)", Default: 0},
				"ticket":               {Type: "string", Description: "Path to ticket/rd.md", Default: "rd.md"},
				"approved_sha":         {Type: "string", Description: "Approved SHA; verbatim patch-id (review/drift)"},
				"approved_base":        {Type: "string", Description: "Base of the approved range; default: merge-base(approved, base)"},
				"require_approved_sha": {Type: "boolean", Description: "Fails if approved_sha is missing", Default: false},
				"warn_only":            {Type: "boolean", Description: "guard: does not flag failure on SeverityError", Default: false},
				"test_cmd":             {Type: "string", Description: "verify: visible tests"},
				"hidden_test_cmd":      {Type: "string", Description: "verify: testes ocultos"},
				"build_cmd":            {Type: "string", Description: "verify: build command (empty to skip)"},
				"lint_cmd":             {Type: "string", Description: "verify: lint command (empty to skip)"},
				"no_build":             {Type: "boolean", Description: "verify: skips build_cmd", Default: false},
				"no_lint":              {Type: "boolean", Description: "verify: skips lint_cmd", Default: false},
				"ruler":                {Type: "string", Description: "verify: rulers, comma-separated (tests; visual is refused here)", Default: "tests"},
				"typecheck_cmd":        {Type: "string", Description: "verify: typecheck command (empty to skip)"},
				"no_typecheck":         {Type: "boolean", Description: "verify: skips typecheck_cmd", Default: false},
				"checks":               checksProperty,
				"skip_checks":          skipChecksProperty,
				"no_checks":            {Type: "boolean", Description: "verify: skips every verify.checks entry", Default: false},
				"dry_run":              {Type: "boolean", Description: "verify: shows the plan without running", Default: false},
				"max_iter":             {Type: "integer", Description: "verify: maximum iterations", Default: 0},
				"timeout":              {Type: "string", Description: "verify: timeout", Default: "10m"},
				"reviewers":            {Type: "integer", Description: "review: maximum goldfish", Default: 0},
				"standards": {
					Type:        "array",
					Description: "review: roles scope/quality/acceptance",
					Items:       &Property{Type: "string"},
				},
				"env": {
					Type:        "array",
					Description: "Extra keys for the environment allowlist (verify)",
					Items:       &Property{Type: "string"},
				},
				"pass_env": {
					Type:        "array",
					Description: "verify: " + passEnvArgDescription,
					Items:       &Property{Type: "string"},
				},
				"egm_docs": {
					Type:        "array",
					Description: "egm: documents added to the selection",
					Items:       &Property{Type: "string"},
				},
				"strict":            {Type: "boolean", Description: "egm: PASS_PARTIAL and an empty selection fail", Default: false},
				"egm_mode":          {Type: "string", Description: "egm: local (default) or ci; ci needs egm_config, implies strict and runs one fresh round with CI's credential, checked against the committed receipt"},
				"egm_config":        {Type: "string", Description: "egm: absolute path of the egm user config (required by egm_mode ci)"},
				"egm_allow_offline": {Type: "boolean", Description: "egm ci: without a credential, run a deterministic-only round capped at PASS_PARTIAL", Default: false},
			},
			Required: []string{"ops"},
		},
		Handler: handleStack,
	})
}

func resolveToolCWD(args map[string]any) (string, error) {
	if cwd := StringArg(args, "cwd"); cwd != "" {
		return AbsPath(cwd), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

func memStoreFromArgs(args map[string]any) (*mem.Store, string, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, "", err
	}
	dir := mem.LocateRepoToolbeltDir(cwd)
	return mem.NewStore(dir), dir, nil
}

func handleMemAdd(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	rule := StringArg(args, "rule")
	category := StringArg(args, "category")
	if category == "" {
		category = "anti_cheat"
	}
	inv, err := s.Add(category, rule, StringArg(args, "ref"), BoolArg(args, "immutable", false))
	if err != nil {
		return nil, err
	}
	out := mem.RedactInvariant(inv)
	return out, nil
}

func handleMemList(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	list, err := s.List(StringArg(args, "category"), BoolArg(args, "all", false))
	if err != nil {
		return nil, err
	}
	return mem.RedactInvariants(list), nil
}

func handleMemShow(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	inv, err := s.Get(StringArg(args, "id"))
	if err != nil {
		return nil, err
	}
	out := mem.RedactInvariant(inv)
	return out, nil
}

func handleMemEdit(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	fields := mem.EditFields{}
	if v, ok := stringArgPresent(args, "category"); ok {
		fields.Category = &v
	}
	if v, ok := stringArgPresent(args, "rule"); ok {
		fields.Rule = &v
	}
	if v, ok := stringArgPresent(args, "ref"); ok {
		fields.Ref = &v
	}
	inv, err := s.Edit(StringArg(args, "id"), fields, mem.MutateOpts{
		Force:  BoolArg(args, "force", false),
		Reason: StringArg(args, "reason"),
	})
	if err != nil {
		return nil, err
	}
	out := mem.RedactInvariant(inv)
	return out, nil
}

func handleMemRetire(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	inv, err := s.Retire(StringArg(args, "id"), mem.MutateOpts{
		Force:  BoolArg(args, "force", false),
		Reason: StringArg(args, "reason"),
	})
	if err != nil {
		return nil, err
	}
	out := mem.RedactInvariant(inv)
	return out, nil
}

func handleMemSearch(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	list, err := s.Search(StringArg(args, "query"), BoolArg(args, "all", false))
	if err != nil {
		return nil, err
	}
	return mem.RedactInvariants(list), nil
}

func handleMemCheck(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	sha := StringArg(args, "expect_sha")
	all := BoolArg(args, "all", false)
	return memCheckReport(s, sha, all)
}

func memCheckReport(s *mem.Store, sha string, includeRetired bool) (*mem.CheckReport, error) {
	list, err := s.List("", includeRetired)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, inv := range list {
		ids = append(ids, inv.ID)
	}
	return s.Check(sha, ids, includeRetired)
}

func stringArgPresent(args map[string]any, key string) (string, bool) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", false
	}
	s, ok := raw.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func handleMemInit(_ context.Context, args map[string]any) (any, error) {
	_, dir, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	result, err := mem.EnsureDefaultAntiCheat(dir)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func handleVerifyRun(ctx context.Context, args map[string]any) (any, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	names, err := verify.ParseRulers(StringArg(args, "ruler"))
	if err != nil {
		return nil, err
	}
	cfg, err := verifyConfigFromArgs(cwd, args)
	if err != nil {
		return nil, err
	}
	multi, err := verify.RunRulers(ctx, &cfg, names)
	if err != nil {
		return nil, err
	}
	// A missing command stays a tool error, as with verify.Run before.
	for _, rr := range multi.Results {
		if res, ok := rr.Payload.(*verify.Result); ok && rr.ExitCode == 127 {
			return nil, errors.New(res.Error)
		}
	}
	return multi.Payload(), nil
}

// verifyConfigFromArgs mirrors tb verify run: R0 and the overrides are
// applied by verify.RunRulers.
func verifyConfigFromArgs(cwd string, args map[string]any) (verify.Config, error) {
	cfg := verify.DefaultConfig(cwd)
	trusted, err := config.LoadTrusted()
	if err != nil {
		return cfg, err
	}
	cfg.TrustedConfig = trusted
	if fileCfg, loadErr := config.Load(); loadErr == nil {
		cfg.EnvAllow = append(cfg.EnvAllow, fileCfg.Env.Allow...)
		cfg.EnvPass = append(cfg.EnvPass, fileCfg.Env.Pass...)
	}
	if v := StringArg(args, "base"); v != "" {
		cfg.BaseRef = v
	}
	if v := StringArg(args, "head"); v != "" {
		cfg.HeadRef = v
	}
	o, err := verifyOverridesFromArgs(args)
	if err != nil {
		return cfg, err
	}
	// The hidden command only overrides when non-empty, as before.
	o.HiddenTestCmd.Set = o.HiddenTestCmd.Cmd != ""
	cfg.Overrides = o
	cfg.DryRun = BoolArg(args, "dry_run", false)
	if n := IntArg(args, "max_iter", 0); n > 0 {
		cfg.MaxIterations = n
	}
	timeoutStr := StringArg(args, "timeout")
	if timeoutStr == "" {
		timeoutStr = "10m"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return cfg, fmt.Errorf("invalid timeout: %w", err)
	}
	cfg.Timeout = timeout
	cfg.EnvAllow = append(cfg.EnvAllow, StringSliceArg(args, "env", nil)...)
	pass, err := toolPassEnv(args)
	if err != nil {
		return cfg, err
	}
	cfg.EnvPass = append(cfg.EnvPass, pass...)
	cfg.Visual = &verify.VisualConfig{
		Manifest:  StringArg(args, "manifest"),
		Threshold: FloatArg(args, "threshold", 0.95),
		SkipOK:    BoolArg(args, "skip_unavailable", true),
	}
	return cfg, nil
}

// verifyOverridesFromArgs reads the command arguments shared by
// tb_verify_run and tb_stack; a present but empty build/lint/typecheck
// command disables that step.
func verifyOverridesFromArgs(args map[string]any) (*verify.FlagOverrides, error) {
	cmdArg := func(key string) verify.CmdOverride {
		return verify.CmdOverride{Cmd: StringArg(args, key), Set: HasArg(args, key)}
	}
	o := &verify.FlagOverrides{
		TestCmd:       cmdArg("test_cmd"),
		HiddenTestCmd: cmdArg("hidden_test_cmd"),
		BuildCmd:      cmdArg("build_cmd"),
		LintCmd:       cmdArg("lint_cmd"),
		TypecheckCmd:  cmdArg("typecheck_cmd"),
		NoBuild:       BoolArg(args, "no_build", false),
		NoLint:        BoolArg(args, "no_lint", false),
		NoTypecheck:   BoolArg(args, "no_typecheck", false),
		SkipChecks:    StringSliceArg(args, "skip_checks", nil),
		NoChecks:      BoolArg(args, "no_checks", false),
	}
	checks, err := checksArg(args)
	if err != nil {
		return nil, err
	}
	o.Checks = checks
	return o, nil
}

// checksArg decodes "checks": an array of {name, cmd} objects.
func checksArg(args map[string]any) ([]config.VerifyCheck, error) {
	raw, ok := args["checks"]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, &verify.UsageError{Msg: "checks must be an array of {name, cmd} objects"}
	}
	checks := make([]config.VerifyCheck, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, &verify.UsageError{Msg: fmt.Sprintf("checks[%d] must be an object {name, cmd}", i)}
		}
		name, _ := obj["name"].(string)
		cmdLine, _ := obj["cmd"].(string)
		name, cmdLine = strings.TrimSpace(name), strings.TrimSpace(cmdLine)
		if name == "" || cmdLine == "" {
			return nil, &verify.UsageError{Msg: fmt.Sprintf("checks[%d] needs a non-empty name and cmd", i)}
		}
		checks = append(checks, config.VerifyCheck{Name: name, Cmd: cmdLine})
	}
	return checks, nil
}

func handleReview(_ context.Context, args map[string]any) (any, error) {
	cfg, err := reviewConfigFromArgs(args)
	if err != nil {
		return nil, err
	}
	return review.Run(cfg)
}

func handleDrift(_ context.Context, args map[string]any) (any, error) {
	cfg, err := driftConfigFromArgs(args)
	if err != nil {
		return nil, err
	}
	rep, err := review.Run(cfg)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"drift":           rep.Drift,
		"has_drift":       driftHasProblems(rep),
		"pinned_approval": rep.PinnedApproval,
	}, nil
}

func driftConfigFromArgs(args map[string]any) (*review.Config, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	ticketPath := StringArg(args, "ticket")
	if ticketPath == "" {
		ticketPath = "rd.md"
	}
	if !filepath.IsAbs(ticketPath) {
		ticketPath = filepath.Join(cwd, ticketPath)
	}
	base := StringArg(args, "base")
	baseSet := base != ""
	if base == "" {
		base = "HEAD~1"
	}
	head := StringArg(args, "head")
	if head == "" {
		head = "HEAD"
	}
	ticket, err := review.LoadTicket(ticketPath)
	if err != nil {
		return nil, err
	}
	if !baseSet && ticket != nil && ticket.BaseCommit != "" {
		base = ticket.BaseCommit
	}
	cfg := review.DefaultConfig(cwd)
	cfg.BaseRef = base
	cfg.BaseExplicit = baseSet
	cfg.HeadRef = head
	cfg.ApprovedSHA = StringArg(args, "approved_sha")
	cfg.ApprovedBase = StringArg(args, "approved_base")
	cfg.RequireApprovedSHA = requireApprovedSHAArg(args)
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	return &cfg, nil
}

func driftHasProblems(rep *review.Report) bool {
	if rep == nil {
		return false
	}
	if rep.PinnedApproval.Diverged() {
		return true
	}
	d := rep.Drift
	return d != nil && (len(d.OutOfScope) > 0 || len(d.ForbiddenHit) > 0 || d.BaseMismatch)
}

func reviewConfigFromArgs(args map[string]any) (*review.Config, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	cfg := review.DefaultConfig(cwd)
	if v := StringArg(args, "base"); v != "" {
		cfg.BaseRef = v
		cfg.BaseExplicit = true
	}
	if v := StringArg(args, "head"); v != "" {
		cfg.HeadRef = v
	}
	cfg.ApprovedSHA = StringArg(args, "approved_sha")
	cfg.ApprovedBase = StringArg(args, "approved_base")
	cfg.RequireApprovedSHA = requireApprovedSHAArg(args)
	ticket := StringArg(args, "ticket")
	if ticket == "" {
		ticket = "rd.md"
	}
	if !filepath.IsAbs(ticket) {
		ticket = filepath.Join(cwd, ticket)
	}
	cfg.TicketPath = ticket
	if n := IntArg(args, "reviewers", 0); n > 0 {
		cfg.Reviewers = n
	}
	cfg.Standards = StringSliceArg(args, "standards", []string{"scope", "quality", "acceptance"})
	return &cfg, nil
}

func requireApprovedSHAArg(args map[string]any) bool {
	if BoolArg(args, "require_approved_sha", false) {
		return true
	}
	fileCfg, err := config.Load()
	if err != nil {
		return false
	}
	return fileCfg.Review.RequireApprovedSHA
}

func splitCSVArg(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func handleGuardCheck(_ context.Context, args map[string]any) (any, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	base, head, err := resolveGuardRefs(args)
	if err != nil {
		return nil, err
	}
	res, err := guard.NewChecker(cwd).Check(base, head)
	if err != nil {
		return nil, err
	}
	warnOnly := BoolArg(args, "warn_only", false)
	strictFail := hasGuardStrictError(res.Violations) && !warnOnly
	return map[string]any{
		"base_ref":      res.BaseRef,
		"head_ref":      res.HeadRef,
		"rules_checked": res.RulesChecked,
		"violations":    res.Violations,
		"strict_fail":   strictFail,
	}, nil
}

func resolveGuardRefs(args map[string]any) (base, head string, err error) {
	if rng := StringArg(args, "range"); rng != "" {
		parts := strings.SplitN(rng, "..", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("use range base..head or base/head")
		}
		return parts[0], parts[1], nil
	}
	base = StringArg(args, "base")
	head = StringArg(args, "head")
	if base == "" {
		base = "HEAD~1"
	}
	if head == "" {
		head = "HEAD"
	}
	return base, head, nil
}

func hasGuardStrictError(violations []guard.Violation) bool {
	for _, v := range violations {
		if v.Severity == guard.SeverityError {
			return true
		}
	}
	return false
}

func handleStack(ctx context.Context, args map[string]any) (any, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	if c := StringArg(args, "egm_config"); c != "" && !filepath.IsAbs(c) {
		return nil, fmt.Errorf("egm_config must be an absolute path")
	}
	opNames := StringSliceArg(args, "ops", nil)
	ops, err := stack.ParseOps(opNames)
	if err != nil {
		return nil, err
	}
	base := StringArg(args, "base")
	if base == "" {
		base = "HEAD~1"
	}
	head := StringArg(args, "head")
	if head == "" {
		head = "HEAD"
	}
	timeoutStr := StringArg(args, "timeout")
	if timeoutStr == "" {
		timeoutStr = "10m"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}
	pass, err := toolPassEnv(args)
	if err != nil {
		return nil, err
	}
	ticket := StringArg(args, "ticket")
	if ticket == "" {
		ticket = "rd.md"
	}
	if !filepath.IsAbs(ticket) {
		ticket = filepath.Join(cwd, ticket)
	}
	vo, err := verifyOverridesFromArgs(args)
	if err != nil {
		return nil, err
	}
	opts := &stack.Options{
		RepoDir:            cwd,
		Base:               base,
		Head:               head,
		Ops:                ops,
		FailFast:           BoolArg(args, "fail_fast", false),
		Jobs:               IntArg(args, "jobs", 0),
		Ticket:             ticket,
		ApprovedSHA:        StringArg(args, "approved_sha"),
		ApprovedBase:       StringArg(args, "approved_base"),
		RequireApprovedSHA: requireApprovedSHAArg(args),
		WarnOnly:           BoolArg(args, "warn_only", false),
		TestCmd:            StringArg(args, "test_cmd"),
		HiddenTestCmd:      StringArg(args, "hidden_test_cmd"),
		BuildCmd:           StringArg(args, "build_cmd"),
		LintCmd:            StringArg(args, "lint_cmd"),
		BuildCmdSet:        HasArg(args, "build_cmd"),
		LintCmdSet:         HasArg(args, "lint_cmd"),
		NoBuild:            BoolArg(args, "no_build", false),
		NoLint:             BoolArg(args, "no_lint", false),
		Ruler:              StringArg(args, "ruler"),
		TypecheckCmd:       vo.TypecheckCmd.Cmd,
		TypecheckCmdSet:    vo.TypecheckCmd.Set,
		NoTypecheck:        vo.NoTypecheck,
		Checks:             vo.Checks,
		SkipChecks:         vo.SkipChecks,
		NoChecks:           vo.NoChecks,
		DryRun:             BoolArg(args, "dry_run", false),
		MaxIter:            IntArg(args, "max_iter", 0),
		Timeout:            timeout,
		Reviewers:          IntArg(args, "reviewers", 0),
		Standards:          StringSliceArg(args, "standards", nil),
		EnvAllow:           StringSliceArg(args, "env", nil),
		EnvPass:            pass,
		EgmDocs:            StringSliceArg(args, "egm_docs", nil),
		Strict:             BoolArg(args, "strict", false),
		EgmMode:            StringArg(args, "egm_mode"),
		EgmConfig:          StringArg(args, "egm_config"),
		EgmAllowOffline:    BoolArg(args, "egm_allow_offline", false),
	}
	return stack.Run(ctx, opts)
}
