package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/stack"
)

var stackCmd = &cobra.Command{
	Use:   "stack <op> [<op>...]",
	Short: "Empilha operações de verificação (verify/review/drift/guard), não branches",
	Long: `Empilha operações de verificação numa execução com contexto compartilhado
(diff, índice de arquivos alterados, memória e config).

Não é stack de branches (como Graphite ou git-stack): só orquestra verify,
review, drift e/ou guard na ordem dada.

Operações: verify, review, drift, guard.
Etapas somente leitura (guard, drift, review) rodam em paralelo; verify fica isolado
(build/test). A saída JSON preserva a ordem pedida. --jobs N limita a concorrência
(default = nº de CPUs, máx. 32; --jobs 1 = serial). --fail-fast cancela as demais
via context. A saída é sempre um único JSON agregado.
Sem operação, sai com erro curto listando as ops válidas (sem dump de Usage).`,
	RunE: runStack,
}

func init() {
	stackCmd.Flags().String("base", "HEAD~1", "ref base compartilhada")
	stackCmd.Flags().String("head", "HEAD", "ref head compartilhada")
	stackCmd.Flags().Bool("fail-fast", false, "cancela as demais etapas na primeira falha")
	stackCmd.Flags().Int("jobs", 0, "limite de concorrência (0=nº de CPUs, máx. 32; 1=serial)")
	stackCmd.Flags().String("ticket", "rd.md", "caminho do ticket/rd.md (review/drift/verify)")
	stackCmd.Flags().String("approved-sha", "", "SHA aprovado (review/drift)")
	stackCmd.Flags().Bool("warn-only", false, "guard: não falha exit em SeverityError")
	stackCmd.Flags().String("test-cmd", "", "verify: linha de testes visíveis")
	stackCmd.Flags().String("hidden-test-cmd", "", "verify: linha de testes ocultos")
	stackCmd.Flags().Int("max-iter", 0, "verify: máximo de iterações")
	stackCmd.Flags().String("timeout", "10m", "verify: timeout")
	stackCmd.Flags().Int("reviewers", 0, "review: máximo de goldfish")
	stackCmd.Flags().String("standards", "", "review: papéis CSV (scope,quality,acceptance)")
}

func runStack(cmd *cobra.Command, args []string) error {
	ops, err := stack.ParseOps(args)
	if err != nil {
		op := ""
		if len(args) > 0 {
			op = args[0]
		}
		return emitStackResult(&stack.Result{
			OK: false,
			Steps: []stack.StepResult{{
				Op:       op,
				OK:       false,
				ExitCode: 1,
				Error:    err.Error(),
			}},
		})
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	timeoutStr, _ := cmd.Flags().GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("timeout inválido: %w", err)
	}
	base, _ := cmd.Flags().GetString("base")
	head, _ := cmd.Flags().GetString("head")
	failFast, _ := cmd.Flags().GetBool("fail-fast")
	jobs, _ := cmd.Flags().GetInt("jobs")
	ticket, _ := cmd.Flags().GetString("ticket")
	approved, _ := cmd.Flags().GetString("approved-sha")
	warnOnly, _ := cmd.Flags().GetBool("warn-only")
	testCmd, _ := cmd.Flags().GetString("test-cmd")
	hiddenCmd, _ := cmd.Flags().GetString("hidden-test-cmd")
	maxIter, _ := cmd.Flags().GetInt("max-iter")
	reviewers, _ := cmd.Flags().GetInt("reviewers")
	standardsCSV, _ := cmd.Flags().GetString("standards")

	opts := &stack.Options{
		RepoDir:       cwd,
		Base:          base,
		Head:          head,
		Ops:           ops,
		FailFast:      failFast,
		Jobs:          jobs,
		Ticket:        ticket,
		ApprovedSHA:   approved,
		WarnOnly:      warnOnly,
		TestCmd:       testCmd,
		HiddenTestCmd: hiddenCmd,
		MaxIter:       maxIter,
		Timeout:       timeout,
		Reviewers:     reviewers,
		Standards:     splitCSV(standardsCSV),
		EnvAllow:      envAllow,
		EnvPass:       envPass,
	}
	res, err := stack.Run(context.Background(), opts)
	if err != nil {
		return err
	}
	return emitStackResult(res)
}

func emitStackResult(res *stack.Result) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	if code := stack.ExitCode(res); code != 0 {
		os.Exit(code)
	}
	return nil
}
