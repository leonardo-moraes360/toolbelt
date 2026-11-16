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
}

func registerMemTools() {
	Register(&Tool{
		Name:        "tb_mem_add",
		Description: "Adiciona invariante em .toolbelt/invariants.yaml (paridade com tb mem add)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"rule":     {Type: "string", Description: "Texto da invariante"},
				"category": {Type: "string", Description: "anti_cheat|mandate|security", Default: "anti_cheat"},
				"cwd":      {Type: "string", Description: "Diretório do repositório"},
			},
			Required: []string{"rule"},
		},
		Handler: handleMemAdd,
	})
	Register(&Tool{
		Name:        "tb_mem_list",
		Description: "Lista invariantes (paridade com tb mem list)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"category": {Type: "string", Description: "Filtrar categoria"},
				"cwd":      {Type: "string", Description: "Diretório do repositório"},
			},
		},
		Handler: handleMemList,
	})
	Register(&Tool{
		Name:        "tb_mem_check",
		Description: "Verifica integridade das invariantes (paridade com tb mem check)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"expect_sha": {Type: "string", Description: "SHA256 esperado"},
				"cwd":        {Type: "string", Description: "Diretório do repositório"},
			},
		},
		Handler: handleMemCheck,
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
				"max_iter":         {Type: "string", Description: "Máximo de iterações", Default: "0"},
				"timeout":          {Type: "string", Description: "Timeout", Default: "10m"},
				"ruler":            {Type: "string", Description: "tests|visual", Default: "tests"},
				"manifest":         {Type: "string", Description: "Manifesto visual"},
				"threshold":        {Type: "string", Description: "Threshold visual", Default: "0.95"},
				"skip_unavailable": {Type: "string", Description: "true/false", Default: "true"},
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
				"cwd":       {Type: "string", Description: "Diretório do repositório"},
				"ticket":    {Type: "string", Description: "Caminho do ticket/rd.md", Default: "rd.md"},
				"base":      {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":      {Type: "string", Description: "Ref head", Default: "HEAD"},
				"reviewers": {Type: "string", Description: "Máximo de goldfish distintos", Default: "3"},
				"standards": {Type: "string", Description: "CSV de papéis: scope,quality,acceptance", Default: "scope,quality,acceptance"},
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
				"cwd":    {Type: "string", Description: "Diretório do repositório"},
				"ticket": {Type: "string", Description: "Caminho do ticket/rd.md", Default: "rd.md"},
				"base":   {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":   {Type: "string", Description: "Ref head", Default: "HEAD"},
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
				"cwd":    {Type: "string", Description: "Diretório do repositório"},
				"base":   {Type: "string", Description: "Ref base", Default: "HEAD~1"},
				"head":   {Type: "string", Description: "Ref head", Default: "HEAD"},
				"range":  {Type: "string", Description: "Intervalo base..head (alternativa a base/head)"},
				"strict": {Type: "string", Description: "true para marcar strict_fail em erros", Default: "false"},
			},
		},
		Handler: handleGuardCheck,
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
	return s.Add(category, rule)
}

func handleMemList(_ context.Context, args map[string]any) (any, error) {
	s, _, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	return s.List(StringArg(args, "category"))
}

func handleMemCheck(_ context.Context, args map[string]any) (any, error) {
	s, dir, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	sha := StringArg(args, "expect_sha")
	rep, err := memCheckReport(s, sha)
	if err != nil {
		if initErr := mem.EnsureDefaultAntiCheat(dir); initErr == nil && sha == "" {
			return memCheckReport(s, "")
		}
		return nil, err
	}
	return rep, nil
}

func memCheckReport(s *mem.Store, sha string) (*mem.CheckReport, error) {
	list, err := s.List("")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, inv := range list {
		ids = append(ids, inv.ID)
	}
	return s.Check(sha, ids)
}

func handleMemInit(_ context.Context, args map[string]any) (any, error) {
	_, dir, err := memStoreFromArgs(args)
	if err != nil {
		return nil, err
	}
	if err := mem.EnsureDefaultAntiCheat(dir); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "path": filepath.Join(dir, mem.DefaultInvariantsFile)}, nil
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
	if v := StringArg(args, "max_iter"); v != "" {
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			cfg.MaxIterations = n
		}
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
}

func runVerifyVisual(ctx context.Context, cwd string, args map[string]any) (any, error) {
	threshold := 0.95
	if v := StringArg(args, "threshold"); v != "" {
		_, _ = fmt.Sscanf(v, "%f", &threshold)
	}
	skipOK := StringArg(args, "skip_unavailable") != "false"
	cfg := verify.VisualConfig{
		Manifest:  StringArg(args, "manifest"),
		Threshold: threshold,
		SkipOK:    skipOK,
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
	if base == "" {
		base = "HEAD~1"
	}
	head := StringArg(args, "head")
	if head == "" {
		head = "HEAD"
	}
	ticket, err := review.LoadTicket(ticketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if ticket != nil && ticket.BaseCommit != "" && base == "HEAD~1" {
		base = ticket.BaseCommit
	}
	cfg := review.DefaultConfig(cwd)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := review.Run(&cfg)
	if err != nil {
		return nil, err
	}
	drift := rep.Drift
	hasDrift := drift != nil && (len(drift.OutOfScope) > 0 || len(drift.ForbiddenHit) > 0 || drift.BaseMismatch)
	return map[string]any{
		"drift":     drift,
		"has_drift": hasDrift,
	}, nil
}

func reviewConfigFromArgs(args map[string]any) (*review.Config, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	cfg := review.DefaultConfig(cwd)
	if v := StringArg(args, "base"); v != "" {
		cfg.BaseRef = v
	}
	if v := StringArg(args, "head"); v != "" {
		cfg.HeadRef = v
	}
	ticket := StringArg(args, "ticket")
	if ticket == "" {
		ticket = "rd.md"
	}
	if !filepath.IsAbs(ticket) {
		ticket = filepath.Join(cwd, ticket)
	}
	cfg.TicketPath = ticket
	if v := StringArg(args, "reviewers"); v != "" {
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			cfg.Reviewers = n
		}
	}
	stds := StringArg(args, "standards")
	if stds == "" {
		stds = "scope,quality,acceptance"
	}
	cfg.Standards = splitCSVArg(stds)
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
	strictFail := StringArg(args, "strict") == "true" && hasGuardStrictError(res.Violations)
	return map[string]any{
		"base_ref":    res.BaseRef,
		"head_ref":    res.HeadRef,
		"violations":  res.Violations,
		"strict_fail": strictFail,
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
