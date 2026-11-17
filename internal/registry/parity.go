package registry

import (
	"context"
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
}

func registerMemTools() {
	registerMemMutateTools()
	registerMemQueryTools()
}

func registerMemMutateTools() {
	Register(&Tool{
		Name:        "tb_mem_add",
		Description: "Adiciona invariante em .toolbelt/invariants.yaml (paridade com tb mem add)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"rule":     {Type: "string", Description: "Texto da invariante"},
				"category": {Type: "string", Description: "anti_cheat|mandate|security", Default: "anti_cheat"},
				"ref":      {Type: "string", Description: "Caminho ADR/decisions.md"},
				"cwd":      {Type: "string", Description: "Diretório do repositório"},
			},
			Required: []string{"rule"},
		},
		Handler: handleMemAdd,
	})
	Register(&Tool{
		Name:        "tb_mem_edit",
		Description: "Edita campos de uma invariante (paridade com tb mem edit)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":       {Type: "string", Description: "ID da invariante"},
				"category": {Type: "string", Description: "Nova categoria"},
				"rule":     {Type: "string", Description: "Nova regra"},
				"ref":      {Type: "string", Description: "Novo caminho ADR/decisions.md"},
				"cwd":      {Type: "string", Description: "Diretório do repositório"},
			},
			Required: []string{"id"},
		},
		Handler: handleMemEdit,
	})
	Register(&Tool{
		Name:        "tb_mem_retire",
		Description: "Aposenta invariante (soft-retire; paridade com tb mem retire)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":  {Type: "string", Description: "ID da invariante"},
				"cwd": {Type: "string", Description: "Diretório do repositório"},
			},
			Required: []string{"id"},
		},
		Handler: handleMemRetire,
	})
	Register(&Tool{
		Name:        "tb_mem_init",
		Description: "Cria invariantes padrão anti-trapaça (paridade com tb mem init)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd": {Type: "string", Description: "Diretório do repositório"},
			},
		},
		Handler: handleMemInit,
	})
}

func registerMemQueryTools() {
	Register(&Tool{
		Name:        "tb_mem_list",
		Description: "Lista invariantes (paridade com tb mem list)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"category": {Type: "string", Description: "Filtrar categoria"},
				"all":      {Type: "boolean", Description: "true para incluir aposentadas", Default: false},
				"cwd":      {Type: "string", Description: "Diretório do repositório"},
			},
		},
		Handler: handleMemList,
	})
	Register(&Tool{
		Name:        "tb_mem_show",
		Description: "Exibe invariante por id (paridade com tb mem show)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"id":  {Type: "string", Description: "ID da invariante"},
				"cwd": {Type: "string", Description: "Diretório do repositório"},
			},
			Required: []string{"id"},
		},
		Handler: handleMemShow,
	})
	Register(&Tool{
		Name:        "tb_mem_search",
		Description: "Busca invariantes (paridade com tb mem search)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Texto a buscar em id/categoria/regra/ref"},
				"all":   {Type: "boolean", Description: "true para incluir aposentadas", Default: false},
				"cwd":   {Type: "string", Description: "Diretório do repositório"},
			},
			Required: []string{"query"},
		},
		Handler: handleMemSearch,
	})
	Register(&Tool{
		Name:        "tb_mem_check",
		Description: "Verifica integridade das invariantes (paridade com tb mem check)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"expect_sha": {Type: "string", Description: "SHA256 esperado"},
				"all":        {Type: "boolean", Description: "true para incluir aposentadas", Default: false},
				"cwd":        {Type: "string", Description: "Diretório do repositório"},
			},
		},
		Handler: handleMemCheck,
	})
}

func registerVerifyTools() {
	Register(&Tool{
		Name:        "tb_verify_run",
		Description: "Executa loop de verificação (paridade com tb verify run)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":              {Type: "string", Description: "Diretório do repositório"},
				"base":             {Type: "string", Description: "Ref base"},
				"head":             {Type: "string", Description: "Ref head"},
				"test_cmd":         {Type: "string", Description: "Linha de testes visíveis"},
				"hidden_test_cmd":  {Type: "string", Description: "Linha de testes ocultos"},
				"max_iter":         {Type: "integer", Description: "Máximo de iterações", Default: 0},
				"timeout":          {Type: "string", Description: "Timeout", Default: "10m"},
				"ruler":            {Type: "string", Description: "tests|visual", Default: "tests"},
				"manifest":         {Type: "string", Description: "Manifesto visual"},
				"threshold":        {Type: "number", Description: "Threshold visual", Default: 0.95},
				"skip_unavailable": {Type: "boolean", Description: "Skip gracioso se ruler indisponível", Default: true},
				"env": {
					Type:        "array",
					Description: "Chaves extras na allowlist de ambiente",
					Items:       &Property{Type: "string"},
				},
				"pass_env": {
					Type:        "array",
					Description: "Opt-in explícito para segredos (*TOKEN*/SECRET/KEY/PASSWORD)",
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
		Description: "Revisão fresh-context contra ticket (paridade com tb review)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":          {Type: "string", Description: "Diretório do repositório"},
				"ticket":       {Type: "string", Description: "Caminho do ticket/rd.md", Default: "rd.md"},
				"base":         {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":         {Type: "string", Description: "Ref head", Default: "HEAD"},
				"approved_sha": {Type: "string", Description: "SHA aprovado para comparar patch-id estável com o head"},
				"reviewers":    {Type: "integer", Description: "Máximo de goldfish distintos", Default: 3},
				"standards": {
					Type:        "array",
					Description: "Papéis: scope,quality,acceptance (aceita CSV legado)",
					Default:     []string{"scope", "quality", "acceptance"},
					Items:       &Property{Type: "string"},
				},
			},
		},
		Handler: handleReview,
	})
	Register(&Tool{
		Name:        "tb_drift",
		Description: "Detecta drift de escopo/base (paridade com tb drift)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":          {Type: "string", Description: "Diretório do repositório"},
				"ticket":       {Type: "string", Description: "Caminho do ticket/rd.md", Default: "rd.md"},
				"base":         {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":         {Type: "string", Description: "Ref head", Default: "HEAD"},
				"approved_sha": {Type: "string", Description: "SHA aprovado para comparar patch-id estável com o head"},
			},
		},
		Handler: handleDrift,
	})
}

func registerGuardTools() {
	Register(&Tool{
		Name:        "tb_guard_check",
		Description: "Analisa diff anti-trapaça (paridade com tb guard check)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":       {Type: "string", Description: "Diretório do repositório"},
				"base":      {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":      {Type: "string", Description: "Ref head", Default: "HEAD"},
				"range":     {Type: "string", Description: "Intervalo base..head (alternativa a base/head)"},
				"warn_only": {Type: "boolean", Description: "true para não marcar strict_fail em SeverityError (opt-out)", Default: false},
				"strict":    {Type: "boolean", Description: "deprecated: strict_fail já é o default em SeverityError; use warn_only=true para opt-out", Default: false},
			},
		},
		Handler: handleGuardCheck,
	})
}

func registerStackTools() {
	Register(&Tool{
		Name:        "tb_stack",
		Description: "Empilha verify/review/drift/guard com contexto compartilhado (paridade com tb stack)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ops": {
					Type:        "array",
					Description: "Operações na ordem: verify, review, drift, guard",
					Items:       &Property{Type: "string"},
				},
				"cwd":             {Type: "string", Description: "Diretório do repositório"},
				"base":            {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":            {Type: "string", Description: "Ref head", Default: "HEAD"},
				"fail_fast":       {Type: "boolean", Description: "Para na primeira etapa que falhar", Default: false},
				"ticket":          {Type: "string", Description: "Caminho do ticket/rd.md", Default: "rd.md"},
				"approved_sha":    {Type: "string", Description: "SHA aprovado (review/drift)"},
				"warn_only":       {Type: "boolean", Description: "guard: não marca falha em SeverityError", Default: false},
				"test_cmd":        {Type: "string", Description: "verify: testes visíveis"},
				"hidden_test_cmd": {Type: "string", Description: "verify: testes ocultos"},
				"max_iter":        {Type: "integer", Description: "verify: máximo de iterações", Default: 0},
				"timeout":         {Type: "string", Description: "verify: timeout", Default: "10m"},
				"reviewers":       {Type: "integer", Description: "review: máximo de goldfish", Default: 0},
				"standards": {
					Type:        "array",
					Description: "review: papéis scope/quality/acceptance",
					Items:       &Property{Type: "string"},
				},
				"env": {
					Type:        "array",
					Description: "Chaves extras na allowlist de ambiente (verify)",
					Items:       &Property{Type: "string"},
				},
				"pass_env": {
					Type:        "array",
					Description: "Opt-in explícito para segredos no verify",
					Items:       &Property{Type: "string"},
				},
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
	return s.Add(category, rule, StringArg(args, "ref"))
}

func handleMemList(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	return s.List(StringArg(args, "category"), BoolArg(args, "all", false))
}

func handleMemShow(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	return s.Get(StringArg(args, "id"))
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
	return s.Edit(StringArg(args, "id"), fields)
}

func handleMemRetire(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	return s.Retire(StringArg(args, "id"))
}

func handleMemSearch(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	return s.Search(StringArg(args, "query"), BoolArg(args, "all", false))
}

func handleMemCheck(_ context.Context, args map[string]any) (any, error) {
	s, dir, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	sha := StringArg(args, "expect_sha")
	all := BoolArg(args, "all", false)
	rep, err := memCheckReport(s, sha, all)
	if err != nil {
		if _, initErr := mem.EnsureDefaultAntiCheat(dir); initErr == nil && sha == "" {
			return memCheckReport(s, "", all)
		}
		return nil, err
	}
	return rep, nil
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
	if StringArg(args, "ruler") == "visual" {
		return runVerifyVisual(ctx, cwd, args)
	}
	return runVerifyTests(ctx, cwd, args)
}

func runVerifyTests(ctx context.Context, cwd string, args map[string]any) (any, error) {
	cfg := verify.DefaultConfig(cwd)
	applyVerifyFileConfig(&cfg)
	if v := StringArg(args, "base"); v != "" {
		cfg.BaseRef = v
	}
	if v := StringArg(args, "head"); v != "" {
		cfg.HeadRef = v
	}
	if v := StringArg(args, "test_cmd"); v != "" {
		cfg.VisibleTest = v
	}
	if v := StringArg(args, "hidden_test_cmd"); v != "" {
		cfg.HiddenTest = v
	}
	if n := IntArg(args, "max_iter", 0); n > 0 {
		cfg.MaxIterations = n
	}
	timeoutStr := StringArg(args, "timeout")
	if timeoutStr == "" {
		timeoutStr = "10m"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("timeout inválido: %w", err)
	}
	cfg.Timeout = timeout
	cfg.EnvAllow = append(cfg.EnvAllow, StringSliceArg(args, "env", nil)...)
	cfg.EnvPass = append(cfg.EnvPass, StringSliceArg(args, "pass_env", nil)...)
	return verify.Run(ctx, &cfg)
}

func applyVerifyFileConfig(cfg *verify.Config) {
	fileCfg, err := config.Load()
	if err != nil {
		return
	}
	if fileCfg.Verify.TestCmd != "" {
		cfg.VisibleTest = fileCfg.Verify.TestCmd
	}
	if fileCfg.Verify.HiddenTestCmd != "" {
		cfg.HiddenTest = fileCfg.Verify.HiddenTestCmd
	}
	if fileCfg.Verify.BuildCmd != "" {
		cfg.BuildCmd = fileCfg.Verify.BuildCmd
	}
	if fileCfg.Verify.LintCmd != "" {
		cfg.LintCmd = fileCfg.Verify.LintCmd
	}
	cfg.EnvAllow = append(cfg.EnvAllow, fileCfg.Env.Allow...)
	cfg.EnvPass = append(cfg.EnvPass, fileCfg.Env.Pass...)
}

func runVerifyVisual(ctx context.Context, cwd string, args map[string]any) (any, error) {
	cfg := verify.VisualConfig{
		Manifest:  StringArg(args, "manifest"),
		Threshold: FloatArg(args, "threshold", 0.95),
		SkipOK:    BoolArg(args, "skip_unavailable", true),
	}
	return verify.RunVisual(ctx, cwd, cfg)
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
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	return &cfg, nil
}

func driftHasProblems(rep *review.Report) bool {
	if rep == nil {
		return false
	}
	if rep.PinnedApproval != nil && !rep.PinnedApproval.Match {
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
			return "", "", fmt.Errorf("use range base..head ou base/head")
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
		return nil, fmt.Errorf("timeout inválido: %w", err)
	}
	ticket := StringArg(args, "ticket")
	if ticket == "" {
		ticket = "rd.md"
	}
	if !filepath.IsAbs(ticket) {
		ticket = filepath.Join(cwd, ticket)
	}
	opts := &stack.Options{
		RepoDir:       cwd,
		Base:          base,
		Head:          head,
		Ops:           ops,
		FailFast:      BoolArg(args, "fail_fast", false),
		Ticket:        ticket,
		ApprovedSHA:   StringArg(args, "approved_sha"),
		WarnOnly:      BoolArg(args, "warn_only", false),
		TestCmd:       StringArg(args, "test_cmd"),
		HiddenTestCmd: StringArg(args, "hidden_test_cmd"),
		MaxIter:       IntArg(args, "max_iter", 0),
		Timeout:       timeout,
		Reviewers:     IntArg(args, "reviewers", 0),
		Standards:     StringSliceArg(args, "standards", nil),
		EnvAllow:      StringSliceArg(args, "env", nil),
		EnvPass:       StringSliceArg(args, "pass_env", nil),
	}
	return stack.Run(ctx, opts)
}
