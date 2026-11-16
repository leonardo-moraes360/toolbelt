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
	Short: "Empilha verify/review/drift/guard numa execução com contexto compartilhado",
	Long: `Executa uma ou mais operações em sequência compartilhando o trabalho caro
(diff, índice de arquivos alterados, memória e config).

Operações: verify, review, drift, guard.
A ordem dada é respeitada. Por padrão roda todas; --fail-fast para na primeira falha.
A saída é sempre um único JSON agregado.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStack,
}

func init() {
	stackCmd.Flags().String("base", "HEAD~1", "ref base compartilhada")
	stackCmd.Flags().String("head", "HEAD", "ref head compartilhada")
	stackCmd.Flags().Bool("fail-fast", false, "para na primeira etapa que falhar")
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
		return err
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
