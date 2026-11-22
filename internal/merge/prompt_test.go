//go:build linux

package merge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// fakeAgent writes a /bin/sh agent. body runs after $nonce and $ids (the
// REGION_IDS line) are set from the prompt on stdin.
func fakeAgent(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "agent.sh")
	script := "#!/bin/sh\ninput=$(cat)\nnonce=$(printf '%s\\n' \"$input\" | sed -n 's/^NONCE: //p' | head -n1)\n" + body + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil { //nolint:gosec // test helper script
		t.Fatal(err)
	}
	return p
}

// answerAll resolves region 1 with text (a printf-escaped JSON string body).
func answerAll(text string) string {
	return `printf '{"nonce":"%s","regions":[{"id":1,"text":"` + text + `"}]}' "$nonce"`
}

func userCfg(t *testing.T, agent, extra string) string {
	t.Helper()
	body := extra
	if agent != "" {
		body = "egm:\n  agent_cmd: [\"" + agent + "\"]\n  vendor: custom\n  tools_disabled: true\n  output: raw\n  timeout_per_reviewer: 20s\n" + extra
	}
	p := filepath.Join(t.TempDir(), "user.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func needLandlock(t *testing.T) {
	t.Helper()
	if ok, _, err := sandbox.CheckLandlockSupport(); !ok {
		t.Skip("no Landlock here:", err)
	}
	t.Setenv("TB_MERGE_STATE_DIR", t.TempDir())
}

func propose(t *testing.T, dir, cfg string, extra func(*Options)) (*Result, error) {
	t.Helper()
	o := &Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyPrompt, Config: cfg}
	if extra != nil {
		extra(o)
	}
	return Run(o)
}

func readReceipt(t *testing.T, ref *ReceiptRef) (rc Receipt, raw []byte) {
	t.Helper()
	b, err := os.ReadFile(ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rc); err != nil {
		t.Fatal(err)
	}
	return rc, b
}

// AC11, AC12, AC20: the proposal moves no ref; the apply replays the receipt
// without an agent and reproduces result_tree.
func TestPromptProposalThenApply(t *testing.T) {
	needLandlock(t)
	t.Setenv("CANARY_VAL", "zzcanaryzz")
	dir := conflictRepo(t)
	start := git(t, dir, "rev-parse", "main")
	agent := fakeAgent(t, "echo \"$CANARY_VAL\" >&2\n"+answerAll(`merged\\n`))
	cfg := userCfg(t, agent, "  env: {pass: [CANARY_VAL]}\n")

	res, err := propose(t, dir, cfg, nil) // no --dry-run: still must not apply
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied || res.Receipt == nil || res.ResultTree == "" || git(t, dir, "rev-parse", "main") != start {
		t.Fatalf("proposal: %+v", res)
	}
	if res.Files[0].Status != "prompt" || res.Conflicts[0].Resolution != "prompt" || res.Conflicts[0].Hunks[0].NovelLines != 1 {
		t.Fatalf("report: %+v %+v", res.Files, res.Conflicts)
	}
	checkProposalReceipt(t, res)

	// Apply: the user config has no agent at all, so nothing can be started.
	applied, err := propose(t, dir, userCfg(t, "", ""), func(o *Options) { o.Resolutions = res.Receipt.ID })
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.ResultTree != res.ResultTree || git(t, dir, "rev-parse", "main^{tree}") != res.ResultTree {
		t.Fatalf("apply: %+v", applied)
	}
	if readFile(t, dir, "shared.txt") != "merged\n" {
		t.Fatalf("content %q", readFile(t, dir, "shared.txt"))
	}
	assertNoTempWorktree(t, dir)
}

func checkProposalReceipt(t *testing.T, res *Result) {
	t.Helper()
	rc, raw := readReceipt(t, res.Receipt)
	if strings.Contains(string(raw), "zzcanaryzz") || len(rc.Runs) != 1 || rc.Runs[0].PayloadSHA256 == "" || rc.ResultTree != res.ResultTree {
		t.Fatalf("receipt: %s", raw)
	}
	if fi, _ := os.Stat(res.Receipt.Path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("receipt mode %v", fi.Mode())
	}
	if len(rc.Resolutions) != 1 || rc.Resolutions[0].Rule != "agent" || rc.Resolutions[0].Text != "merged\n" || rc.Resolutions[0].NovelLines != 1 {
		t.Fatalf("resolutions: %+v", rc.Resolutions)
	}
}

// AC13: invalid answers leave the file unresolved with the stated reason.
func TestPromptInvalidAnswers(t *testing.T) {
	needLandlock(t)
	for name, tc := range map[string]struct{ body, reason string }{
		"wrong nonce":   {`printf '{"nonce":"bad","regions":[{"id":1,"text":"x\\n"}]}'`, "agent_protocol"},
		"missing id":    {`printf '{"nonce":"%s","regions":[]}' "$nonce"`, "agent_protocol"},
		"extra id":      {`printf '{"nonce":"%s","regions":[{"id":1,"text":"x\\n"},{"id":2,"text":"y\\n"}]}' "$nonce"`, "agent_protocol"},
		"duplicate id":  {`printf '{"nonce":"%s","regions":[{"id":1,"text":"x\\n"},{"id":1,"text":"y\\n"}]}' "$nonce"`, "agent_protocol"},
		"not json":      {`echo hello`, "agent_protocol"},
		"oversized":     {`printf '{"nonce":"%s","regions":[{"id":1,"text":"' "$nonce"; head -c 9000 /dev/zero | tr '\0' a; printf '"}]}'`, "agent_oversized"},
		"marker":        {answerAll(`<<<<<<< ours\\n`), "agent_markers"},
		"secret":        {answerAll(`API_KEY=abcdef123456\\n`), "agent_secret"},
		"refuse":        {`printf '{"nonce":"%s","refuse":"unsafe"}' "$nonce"`, "agent_refused"},
		"fenced is ok?": {"printf '```json\\n{\"nonce\":\"%s\",\"regions\":[{\"id\":1,\"text\":\"x\\\\n\"}]}\\n```' \"$nonce\"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			dir := conflictRepo(t)
			start := git(t, dir, "rev-parse", "main")
			res, err := propose(t, dir, userCfg(t, fakeAgent(t, tc.body), ""), nil)
			if tc.reason == "" {
				if err != nil || res.Conflicts[0].Resolution != "prompt" {
					t.Fatalf("expected a resolution: %v %+v", err, res)
				}
				return
			}
			if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != tc.reason {
				t.Fatalf("err %v conflicts %+v", err, res)
			}
			rc, _ := readReceipt(t, res.Receipt)
			if len(rc.Unresolved) != 1 || rc.ResultTree != "" || git(t, dir, "rev-parse", "main") != start {
				t.Fatalf("receipt %+v", rc)
			}
			// An incomplete receipt can never be applied.
			_, err = propose(t, dir, userCfg(t, "", ""), func(o *Options) { o.Resolutions = res.Receipt.ID })
			var me *Error
			if !errors.As(err, &me) || me.Reason != "receipt_incomplete" {
				t.Fatalf("apply of incomplete: %v", err)
			}
		})
	}
}

// A Go file whose agent text no longer parses is unresolved (parse_failed).
func TestPromptGoParseFailure(t *testing.T) {
	needLandlock(t)
	dir := newRepo(t)
	src := "package p\n\nfunc f() int { return %d }\n"
	commitFiles(t, dir, map[string]string{"p.go": strings.Replace(src, "%d", "0", 1)})
	addTicket(t, dir, nil, map[string]string{"p.go": strings.Replace(src, "%d", "1", 1)})
	addTicket(t, dir, nil, map[string]string{"p.go": strings.Replace(src, "%d", "2", 1)})
	res, err := propose(t, dir, userCfg(t, fakeAgent(t, answerAll(`func f( {\\n`)), ""), nil)
	if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "parse_failed" {
		t.Fatalf("err %v %+v", err, res)
	}
}

// AC14: deny-listed paths and secrets in the context never reach the agent.
func TestPromptSecretsNeverSent(t *testing.T) {
	needLandlock(t)
	agent := fakeAgent(t, answerAll(`ok\\n`))
	cfg := userCfg(t, agent, "")
	for name, tc := range map[string]struct{ file, one, two, reason string }{
		"denied path":       {"keys.pem", "one\n", "two\n", "secret_path"},
		"secret in region":  {"conf.txt", "one\n", "API_KEY=abcdef123456\n", "secret_in_context"},
		"secret in context": {"ctx.txt", "one\n", "two\n", "secret_in_context"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t)
			pre := ""
			if name == "secret in context" {
				pre = "TOKEN_KEY=abcdef123456\n"
			}
			commitFiles(t, dir, map[string]string{tc.file: pre + "mid\n"})
			addTicket(t, dir, nil, map[string]string{tc.file: pre + tc.one})
			addTicket(t, dir, nil, map[string]string{tc.file: pre + tc.two})
			res, err := propose(t, dir, cfg, nil)
			if !errors.Is(err, ErrConflict) || len(res.Conflicts) == 0 {
				t.Fatalf("err %v %+v", err, res)
			}
			if got := res.Conflicts[0].Reason; got != tc.reason {
				t.Fatalf("reason %q want %q (%+v)", got, tc.reason, res.Conflicts[0])
			}
			if rc, _ := readReceipt(t, res.Receipt); len(rc.Runs) != 0 {
				t.Fatalf("the agent ran: %+v", rc.Runs)
			}
		})
	}
}

// AC15: an apply fails, moving no ref, on a tampered, stale or mismatching receipt.
func TestPromptApplyChecks(t *testing.T) {
	needLandlock(t)
	setup := func(t *testing.T) (dir string, res *Result, cfg string) {
		dir = conflictRepo(t)
		cfg = userCfg(t, fakeAgent(t, answerAll(`merged\\n`)), "")
		res, err := propose(t, dir, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		return dir, res, cfg
	}
	reasonOf := func(err error) string {
		var me *Error
		if errors.As(err, &me) {
			return me.Reason
		}
		return "none: " + err.Error()
	}
	t.Run("tampered", func(t *testing.T) {
		dir, res, cfg := setup(t)
		b, _ := os.ReadFile(res.Receipt.Path)
		if err := os.WriteFile(res.Receipt.Path, []byte(strings.Replace(string(b), "merged", "evil", 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := propose(t, dir, cfg, func(o *Options) { o.Resolutions = res.Receipt.ID })
		if reasonOf(err) != "receipt_tampered" {
			t.Fatal(err)
		}
	})
	t.Run("stale target", func(t *testing.T) {
		dir, res, cfg := setup(t)
		write(t, dir, "other.txt", "o\n")
		git(t, dir, "add", "-A")
		git(t, dir, "commit", "-q", "-m", "moved")
		_, err := propose(t, dir, cfg, func(o *Options) { o.Resolutions = res.Receipt.ID })
		if reasonOf(err) != "receipt_stale" {
			t.Fatal(err)
		}
	})
	t.Run("stale ticket head", func(t *testing.T) {
		dir, res, cfg := setup(t)
		s := ticket.Open(dir)
		if _, err := s.Update("TKT-002", func(x *ticket.Ticket) error {
			x.Result.HeadCommit = git(t, dir, "rev-parse", "work/tkt-001")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		_, err := propose(t, dir, cfg, func(o *Options) { o.Resolutions = res.Receipt.ID })
		if reasonOf(err) != "receipt_stale" {
			t.Fatal(err)
		}
	})
	t.Run("input hash mismatch", func(t *testing.T) {
		dir, res, cfg := setup(t)
		rc, _ := readReceipt(t, res.Receipt)
		rc.Resolutions[0].OursSHA256 = hexSum("other")
		id := rewriteReceipt(t, res.Receipt, &rc)
		_, err := propose(t, dir, cfg, func(o *Options) { o.Resolutions = id })
		if reasonOf(err) != "receipt_mismatch" {
			t.Fatal(err)
		}
	})
	t.Run("tree mismatch", func(t *testing.T) {
		dir, res, cfg := setup(t)
		rc, _ := readReceipt(t, res.Receipt)
		rc.ResultTree = strings.Repeat("0", 40)
		id := rewriteReceipt(t, res.Receipt, &rc)
		start := git(t, dir, "rev-parse", "main")
		_, err := propose(t, dir, cfg, func(o *Options) { o.Resolutions = id })
		if reasonOf(err) != "receipt_mismatch" || git(t, dir, "rev-parse", "main") != start {
			t.Fatal(err)
		}
	})
}

// rewriteReceipt stores a modified receipt next to the original under its own
// hash, so only the content checks (not the tamper check) can catch it.
func rewriteReceipt(t *testing.T, orig *ReceiptRef, rc *Receipt) string {
	t.Helper()
	b, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	id := hexSum(string(b))[:16]
	if err := os.WriteFile(filepath.Join(filepath.Dir(orig.Path), id+".json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

// AC17: a timeout is retried once; an escaped process aborts without a receipt.
func TestPromptRunFailures(t *testing.T) {
	needLandlock(t)
	t.Run("timeout retried", func(t *testing.T) {
		dir := conflictRepo(t)
		cfg := userCfg(t, fakeAgent(t, "sleep 30"), "merge:\n  prompt:\n    timeout_per_file: 1s\n")
		start := time.Now()
		res, err := propose(t, dir, cfg, nil)
		if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "timeout" {
			t.Fatalf("err %v %+v", err, res)
		}
		if rc, _ := readReceipt(t, res.Receipt); len(rc.Runs) != 2 || time.Since(start) > 20*time.Second {
			t.Fatalf("runs %+v", rc.Runs)
		}
	})
	t.Run("exit retried then reported", func(t *testing.T) {
		dir := conflictRepo(t)
		res, err := propose(t, dir, userCfg(t, fakeAgent(t, "exit 3"), ""), nil)
		if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "exit_3" {
			t.Fatalf("err %v %+v", err, res)
		}
	})
	t.Run("escaped process aborts", func(t *testing.T) {
		dir := conflictRepo(t)
		start := git(t, dir, "rev-parse", "main")
		agent := fakeAgent(t, "setsid sleep 60 >/dev/null 2>&1 &\nsleep 1\n"+answerAll(`merged\\n`))
		res, err := propose(t, dir, userCfg(t, agent, ""), nil)
		var me *Error
		if !errors.As(err, &me) || (me.Reason != "escaped_process" && me.Reason != "unattributed_orphan") || res != nil {
			t.Fatalf("err %v res %v", err, res)
		}
		if git(t, dir, "rev-parse", "main") != start {
			t.Fatal("ref moved")
		}
		assertNoTempWorktree(t, dir)
	})
}

// AC18: without a user egm.agent_cmd prompt fails; the repository's own keys are ignored.
func TestPromptAgentUnavailableAndRepoConfig(t *testing.T) {
	needLandlock(t)
	dir := conflictRepo(t)
	write(t, dir, ".toolbelt/config.yaml", "egm:\n  agent_cmd: [\"/bin/true\"]\nmerge:\n  prompt:\n    max_files: 99\n")
	_, err := propose(t, dir, userCfg(t, "", ""), nil)
	var me *Error
	if !errors.As(err, &me) || me.Reason != "agent_unavailable" {
		t.Fatalf("err %v", err)
	}
	// semantic still works and warns about the ignored keys.
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategySemantic, DryRun: true, Config: userCfg(t, "", "")})
	if !errors.Is(err, ErrConflict) || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, "egm.agent_cmd") || !strings.Contains(res.Warnings[0].Message, "merge.prompt.max_files") {
		t.Fatalf("warnings %+v err %v", res, err)
	}
}

// prompt runs semantic first: only the regions the rules left reach the agent.
func TestPromptSemanticFirstAndBudget(t *testing.T) {
	needLandlock(t)
	dir := newRepo(t)
	commitFiles(t, dir, map[string]string{"x.txt": "zero\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nA4\nl5\nl6\nl7\nl8\nl9\n", "x.txt": "one\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nl4\nB5\nl6\nl7\nl8\nl9\n", "x.txt": "two\n"})
	res, err := propose(t, dir, userCfg(t, fakeAgent(t, answerAll(`merged\\n`)), ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]string{}
	for _, rs := range res.Resolutions {
		rules[rs.File] = rs.Rule
	}
	if rules["shared.txt"] != "disjoint" || rules["x.txt"] != "" && rules["x.txt"] != "agent" {
		t.Fatalf("rules %v", rules)
	}
	rc, _ := readReceipt(t, res.Receipt)
	if len(rc.Runs) != 1 || rc.Runs[0].File != "x.txt" {
		t.Fatalf("runs %+v", rc.Runs)
	}

	// merge.prompt.max_files: refused before any run.
	dir = newRepo(t)
	commitFiles(t, dir, map[string]string{"x.txt": "zero\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "one\n", "x.txt": "one\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "two\n", "x.txt": "two\n"})
	cfg := userCfg(t, fakeAgent(t, answerAll(`merged\\n`)), "merge:\n  prompt:\n    max_files: 1\n")
	res, err = propose(t, dir, cfg, nil)
	if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "prompt_budget" {
		t.Fatalf("err %v %+v", err, res)
	}
	if rc, _ := readReceipt(t, res.Receipt); len(rc.Runs) != 0 {
		t.Fatalf("runs %+v", rc.Runs)
	}
}

// Novel lines are reported; max_novel_lines makes them a refusal.
func TestPromptNovelLines(t *testing.T) {
	needLandlock(t)
	dir := conflictRepo(t)
	cfg := userCfg(t, fakeAgent(t, answerAll(`brand new\\n`)), "merge:\n  prompt:\n    max_novel_lines: 0\n")
	res, err := propose(t, dir, cfg, nil)
	if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "agent_novel" {
		t.Fatalf("err %v %+v", err, res)
	}
	dir = conflictRepo(t)
	cfg = userCfg(t, fakeAgent(t, answerAll(`one\\n`)), "merge:\n  prompt:\n    max_novel_lines: 0\n")
	if res, err = propose(t, dir, cfg, nil); err != nil || res.Conflicts[0].Hunks[0].NovelLines != 0 {
		t.Fatalf("reused lines must pass: %v %+v", err, res)
	}
}

func TestBuildPromptShape(t *testing.T) {
	a := &agentEnv{cfg: config.DefaultMergeConfig().Prompt, titles: map[string]string{"TKT-001": "first\nline"}, order: []string{"TKT-001"}}
	rg := &region{ID: 1, Base: []string{"b\n"}, Ours: []string{"o\n"}, Theirs: []string{"t\n"}, OursRange: [2]int{2, 2}}
	st := &fileState{path: "a.txt"}
	st.stages[1].data = []byte("c1\no\nc2\n")
	prompt, nonce, reason := a.buildPrompt(st, []*region{rg})
	if reason != "" || len(nonce) != 32 {
		t.Fatalf("%q %q", reason, nonce)
	}
	for _, want := range []string{"NONCE: " + nonce, "REGION_IDS: 1", "<<<DATA-" + nonce + " REGION 1 OURS>>>\no\n<<<END-" + nonce + ">>>",
		"CONTEXT_BEFORE>>>\nc1\n", "CONTEXT_AFTER>>>\nc2\n", "TKT-001: first line"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	a.cfg.MaxPromptBytes = 100
	if _, _, reason := a.buildPrompt(st, []*region{rg}); reason != "prompt_budget" {
		t.Fatalf("budget: %q", reason)
	}
}

func TestSweepReceipts(t *testing.T) {
	dir := t.TempDir()
	old, fresh := filepath.Join(dir, "0123456789abcdef.json"), filepath.Join(dir, "fedcba9876543210.json")
	for _, p := range []string{old, fresh} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	sweepReceipts(dir, time.Now())
	if _, err := os.Stat(old); err == nil {
		t.Fatal("old receipt kept")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh receipt removed")
	}
}
