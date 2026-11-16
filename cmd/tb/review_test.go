package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/review"
)

func TestPrintReviewReportUsesPapelNotAcordo(t *testing.T) {
	rep := &review.Report{
		Target:    "rd.md",
		BaseRef:   "HEAD~1",
		HeadRef:   "HEAD",
		FreshOK:   true,
		Reviewers: 3,
		Findings: []review.Finding{{
			ID:        "quality-todo-a.go",
			Severity:  review.SeverityWarning,
			Category:  "quality",
			Message:   "TODO/FIXME/XXX/HACK adicionado em comentário no diff",
			File:      "a.go",
			Reviewer:  "goldfish-quality",
			Role:      "quality",
			Agreement: 1,
		}},
	}
	out := captureStdout(t, func() { printReviewReport(rep) })
	if strings.Contains(out, "acordo=") {
		t.Fatalf("não deve imprimir acordo=N/M; got:\n%s", out)
	}
	if !strings.Contains(out, "papel=quality") {
		t.Fatalf("esperado papel=quality; got:\n%s", out)
	}
}

func TestPrintReviewReportPapéisAcordoWhenMultiple(t *testing.T) {
	rep := &review.Report{
		Target:    "rd.md",
		BaseRef:   "HEAD~1",
		HeadRef:   "HEAD",
		FreshOK:   true,
		Reviewers: 3,
		Findings: []review.Finding{{
			ID:        "shared",
			Severity:  review.SeverityWarning,
			Category:  "quality",
			Message:   "mesmo achado",
			Reviewer:  "goldfish-quality",
			Role:      "quality",
			Agreement: 2,
		}},
	}
	out := captureStdout(t, func() { printReviewReport(rep) })
	if !strings.Contains(out, "papel=quality") || !strings.Contains(out, "papéis_acordo=2") {
		t.Fatalf("esperado papel + papéis_acordo; got:\n%s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}
