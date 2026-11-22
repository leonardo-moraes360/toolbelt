package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// joinRole joins an extra agent with the given role.
func joinRole(t *testing.T, s *Store, role Role, name string) Member {
	t.Helper()
	inv, err := s.CreateInvite("feature-x", role, "", 0)
	must(t, err)
	j, err := s.Join("feature-x", inv.Code, &AgentCard{Name: name}, "")
	must(t, err)
	return j.Member
}

func assign(s *Store, m Member, thread, parent string) (SendResult, error) {
	return s.Send(m, &SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: thread, Parent: parent,
		Payload: json.RawMessage(`{"ticket":"` + thread + `"}`)})
}

func mustAssign(t *testing.T, s *Store, m Member, thread, parent string) SendResult {
	t.Helper()
	r, err := assign(s, m, thread, parent)
	must(t, err)
	return r
}

func claim(t *testing.T, s *Store, m Member, thread string) Claim {
	t.Helper()
	c, err := s.ClaimThread(m, thread)
	must(t, err)
	return c
}

func TestParentShapeErrors(t *testing.T) {
	s, _, owner, _ := setup(t)
	cases := []struct {
		in   SendInput
		want error
	}{
		{SendInput{To: "topic:x", Type: TypeQuery, Thread: "A", Parent: "P"}, ErrParentRequiresAssign},
		{SendInput{To: "role:worker", Type: TypeTaskAssign, Parent: "P"}, ErrParentRequiresThread},
		{SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: "P", Parent: "P"}, ErrParentSelf},
	}
	for _, c := range cases {
		_, err := s.Send(owner, &c.in)
		if !errors.Is(err, c.want) || !strings.HasPrefix(err.Error(), c.want.Error()) {
			t.Errorf("%+v: got %v, want %v", c.in, err, c.want)
		}
	}
	_, err := s.Send(owner, &SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: "A", Parent: "bad parent"})
	wantErr(t, err, "invalid parent", "parent syntax")
}

func TestParentOwnership(t *testing.T) {
	s, c, owner, worker := setup(t)
	other := joinRole(t, s, RoleSupervisor, "claude-2")
	router := joinRole(t, s, RoleRouter, "router-1")

	if _, err := assign(s, owner, "C1", "P"); !errors.Is(err, ErrParentUnknown) {
		t.Fatalf("unknown parent: %v", err)
	}
	mustAssign(t, s, owner, "P", "")
	if _, err := assign(s, owner, "C1", "P"); !errors.Is(err, ErrParentNotOwned) {
		t.Fatalf("unclaimed parent: %v", err)
	}
	claim(t, s, owner, "P")
	if _, err := assign(s, other, "C1", "P"); !errors.Is(err, ErrParentNotOwned) {
		t.Fatalf("another supervisor: %v", err)
	}
	// A worker cannot assign at all; the role check still comes first.
	if _, err := assign(s, worker, "C1", "P"); err == nil || errors.Is(err, ErrParentNotOwned) {
		t.Fatalf("worker assign: %v", err)
	}
	if r := mustAssign(t, s, owner, "C1", "P"); r.Message.Parent != "P" {
		t.Fatalf("stored parent: %+v", r.Message)
	}
	// The router links under any known parent without holding it.
	mustAssign(t, s, router, "C3", "P")
	// A supervisor that lost its lease is refused.
	c.add(DefaultLease + time.Second)
	if _, err := s.Watchdog("feature-x", false); err != nil {
		t.Fatal(err)
	}
	if _, err := assign(s, owner, "C9", "P"); !errors.Is(err, ErrParentNotOwned) {
		t.Fatalf("lost lease: %v", err)
	}
}

func TestParentIsSetOnceAndCarried(t *testing.T) {
	s, _, owner, worker := setup(t)
	mustAssign(t, s, owner, "P", "")
	claim(t, s, owner, "P")
	mustAssign(t, s, owner, "C1", "P")
	// A re-send of the same subtask is accepted.
	mustAssign(t, s, owner, "C1", "P")
	// An idempotent duplicate returns the stored message before any parent check.
	idem := SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: "C2", Parent: "P", IdempotencyKey: "k1"}
	a, err := s.Send(owner, &idem)
	must(t, err)
	b, err := s.Send(owner, &idem)
	must(t, err)
	if !b.Duplicate || b.Message.ID != a.Message.ID || b.Message.Parent != "P" {
		t.Fatalf("duplicate: %+v", b)
	}
	mustAssign(t, s, owner, "Q", "")
	claim(t, s, owner, "Q")
	if _, err := assign(s, owner, "C1", "Q"); !errors.Is(err, ErrParentImmutable) {
		t.Fatalf("re-parent: %v", err)
	}
	mustAssign(t, s, owner, "R", "") // a root with an assign and no link
	if _, err := assign(s, owner, "R", "P"); !errors.Is(err, ErrParentImmutable) {
		t.Fatalf("link on an existing root: %v", err)
	}
	// A later parentless assign keeps the link.
	mustAssign(t, s, owner, "C1", "")
	tr, err := s.Tree("feature-x", "P", false, 0)
	must(t, err)
	if len(tr.Nodes) != 3 || tr.Nodes[1].Thread != "C1" || tr.Nodes[1].Parent != "P" {
		t.Fatalf("tree: %+v", tr.Nodes)
	}
	// recv and log carry the field.
	msgs, err := s.Recv(context.Background(), worker, RecvOptions{Peek: true, Limit: 500})
	must(t, err)
	withParent := 0
	for i := range msgs {
		if msgs[i].Parent != "" {
			withParent++
		}
	}
	if withParent == 0 {
		t.Fatal("recv lost the parent field")
	}
	logged, err := s.Log("feature-x", "C2", 10)
	must(t, err)
	if len(logged) != 1 || logged[0].Parent != "P" {
		t.Fatalf("log: %+v", logged)
	}
}

func TestDepthLimit(t *testing.T) {
	s, _, owner, _ := setup(t)
	mustAssign(t, s, owner, "T0", "")
	for i := 1; i <= MaxDepth; i++ {
		claim(t, s, owner, fmt.Sprintf("T%d", i-1))
		mustAssign(t, s, owner, fmt.Sprintf("T%d", i), fmt.Sprintf("T%d", i-1))
	}
	claim(t, s, owner, fmt.Sprintf("T%d", MaxDepth))
	_, err := assign(s, owner, "TX", fmt.Sprintf("T%d", MaxDepth))
	if !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("depth %d child: %v", MaxDepth+1, err)
	}
	tr, err := s.Tree("feature-x", "", false, 0)
	must(t, err)
	if got := tr.Nodes[len(tr.Nodes)-1]; got.Thread != fmt.Sprintf("T%d", MaxDepth) || got.Depth != MaxDepth {
		t.Fatalf("deepest node: %+v", got)
	}
}

func TestFanoutLimit(t *testing.T) {
	s, c, owner, worker := setup(t)
	mustAssign(t, s, owner, "P", "")
	claim(t, s, owner, "P")
	kid := func(i int) string { return fmt.Sprintf("P/%d", i) }
	for i := 1; i < MaxOpenChildren; i++ {
		mustAssign(t, s, owner, kid(i), "P")
	}
	// The last slot is a child that expires unclaimed.
	if _, err := s.Send(owner, &SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: kid(MaxOpenChildren), Parent: "P", TTL: "1m"}); err != nil {
		t.Fatal(err)
	}
	if _, err := assign(s, owner, "P/over", "P"); !errors.Is(err, ErrFanoutExceeded) {
		t.Fatalf("ninth open child: %v", err)
	}
	c.add(2 * time.Minute) // the ttl child expires unclaimed: it stops being open
	mustAssign(t, s, owner, "P/n1", "P")
	if _, err := assign(s, owner, "P/over", "P"); !errors.Is(err, ErrFanoutExceeded) {
		t.Fatalf("full again: %v", err)
	}
	// Completing a child frees a slot.
	wc := claim(t, s, worker, kid(1))
	_, err := s.Complete(worker, kid(1), wc.Epoch, nil)
	must(t, err)
	mustAssign(t, s, owner, "P/n2", "P")
	// So does a revoke.
	if _, err := assign(s, owner, "P/over", "P"); !errors.Is(err, ErrFanoutExceeded) {
		t.Fatalf("full again: %v", err)
	}
	if _, err := s.Revoke(owner, kid(2), "timeout"); err != nil {
		t.Fatal(err)
	}
	mustAssign(t, s, owner, "P/n3", "P")
	// A released child (owner lost its lease) is still open.
	claim(t, s, worker, kid(3))
	c.add(DefaultLease + time.Second)
	_, err = s.Heartbeat(owner)
	must(t, err)
	if _, err := s.Watchdog("feature-x", false); err != nil {
		t.Fatal(err)
	}
	if _, err := assign(s, owner, "P/over", "P"); !errors.Is(err, ErrFanoutExceeded) {
		t.Fatalf("released child must stay open: %v", err)
	}
}

func TestCompleteReportsOpenChildren(t *testing.T) {
	s, _, owner, worker := setup(t)
	mustAssign(t, s, owner, "P", "")
	pc := claim(t, s, owner, "P")
	mustAssign(t, s, owner, "P/ui", "P")
	mustAssign(t, s, owner, "P/api", "P")
	mustAssign(t, s, owner, "P/done", "P")
	dc := claim(t, s, worker, "P/done")
	_, err := s.Complete(worker, "P/done", dc.Epoch, nil)
	must(t, err)
	res, err := s.Complete(owner, "P", pc.Epoch, nil)
	must(t, err)
	if got := strings.Join(res.OpenChildren, ","); got != "P/api,P/ui" {
		t.Fatalf("open children: %q", got)
	}
	if res.Claim.CompletedAt == nil || res.Message.Type != TypeTaskResult {
		t.Fatalf("complete result: %+v", res)
	}
	tr, err := s.Tree("feature-x", "P", false, 0)
	must(t, err)
	if tr.Nodes[0].OpenChildren != len(res.OpenChildren) {
		t.Fatalf("tree open_children %d vs complete %d", tr.Nodes[0].OpenChildren, len(res.OpenChildren))
	}
	// No children: the field is omitted.
	mustAssign(t, s, owner, "S", "")
	sc := claim(t, s, owner, "S")
	res, err = s.Complete(owner, "S", sc.Epoch, nil)
	must(t, err)
	if b, _ := json.Marshal(res); strings.Contains(string(b), "open_children") {
		t.Fatalf("empty open_children serialized: %s", b)
	}
}

// treeFixture builds a tree with every node state, a childless root and an
// older root; payload and result carry marker strings the tree must not leak.
func treeFixture(t *testing.T) (s *Store, c *clock, owner, worker Member) {
	t.Helper()
	s, c, owner, worker = setup(t)
	other := third(t, s, "codex-1")
	if _, err := s.Send(owner, &SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: "P",
		Payload: json.RawMessage(`{"note":"PAYLOAD-SECRET-1"}`)}); err != nil {
		t.Fatal(err)
	}
	claim(t, s, owner, "P")
	for _, th := range []string{"P/claimed", "P/assigned", "P/revoked", "P/completed", "P/released", "P/gone"} {
		mustAssign(t, s, owner, th, "P")
	}
	claim(t, s, worker, "P/claimed")
	if _, err := s.Revoke(owner, "P/revoked", "timeout"); err != nil {
		t.Fatal(err)
	}
	cc := claim(t, s, worker, "P/completed")
	_, err := s.Complete(worker, "P/completed", cc.Epoch, json.RawMessage(`{"out":"RESULT-SECRET-2"}`))
	must(t, err)
	claim(t, s, other, "P/released")
	mustAssign(t, s, owner, "Lone", "") // a root without children
	mustAssign(t, s, owner, "Old", "")  // the newest root
	// Put two nodes into the states the public API reaches only with time:
	// a ttl already past, and a watchdog-style release.
	for _, q := range []string{
		`UPDATE messages SET expires_at = 1 WHERE thread = 'P/gone'`,
		`UPDATE claims SET released_at = 5 WHERE thread = 'P/released'`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	c.add(time.Second)
	return s, c, owner, worker
}

func TestTreeStates(t *testing.T) {
	s, _, owner, worker := treeFixture(t)
	tr, err := s.Tree("feature-x", "", false, 0)
	must(t, err)
	got := map[string]TreeNode{}
	for _, n := range tr.Nodes {
		got[n.Thread] = n
	}
	want := map[string]string{"P": StateClaimed, "P/claimed": StateClaimed, "P/assigned": StateAssigned, "P/revoked": StateRevoked,
		"P/completed": StateCompleted, "P/released": StateReleased, "P/gone": StateExpired}
	if len(tr.Nodes) != len(want) {
		t.Fatalf("default view lists %d nodes: %+v", len(tr.Nodes), tr.Nodes)
	}
	for th, st := range want {
		if got[th].State != st {
			t.Errorf("%s: state %q, want %q", th, got[th].State, st)
		}
	}
	if p := got["P"]; p.OpenChildren != 3 || p.Owner != owner.ID() || p.Epoch != 1 || p.AssignedBy != owner.ID() || p.AssignID == "" || p.Depth != 0 {
		t.Errorf("root node: %+v", p)
	}
	if k := got["P/claimed"]; k.Parent != "P" || k.Depth != 1 || k.Owner != worker.ID() {
		t.Errorf("child node: %+v", k)
	}
}

func TestTreeFilters(t *testing.T) {
	s, _, _, _ := treeFixture(t)
	all, err := s.Tree("feature-x", "", true, 0)
	must(t, err)
	if len(all.Nodes) != 9 {
		t.Fatalf("--all lists %d nodes", len(all.Nodes))
	}
	// Newest root first: Old, Lone, then P.
	if all.Nodes[0].Thread != "Old" || all.Nodes[1].Thread != "Lone" {
		t.Fatalf("root order: %s %s", all.Nodes[0].Thread, all.Nodes[1].Thread)
	}
	lim, err := s.Tree("feature-x", "", true, 1)
	must(t, err)
	if len(lim.Nodes) != 1 || lim.Nodes[0].Thread != "Old" {
		t.Fatalf("--limit 1: %+v", lim.Nodes)
	}
	sub, err := s.Tree("feature-x", "P/claimed", false, 0)
	must(t, err)
	if len(sub.Nodes) != 1 || sub.Nodes[0].Thread != "P/claimed" {
		t.Fatalf("--thread: %+v", sub.Nodes)
	}
	if _, err := s.Tree("feature-x", "nope", false, 0); err == nil {
		t.Fatal("unknown --thread accepted")
	}
	if _, err := s.Tree("nope", "", false, 0); err == nil {
		t.Fatal("unknown channel accepted")
	}
}

func TestTreeJSONHasNoPayloadAndWorksWhenExpired(t *testing.T) {
	s, c, _, _ := treeFixture(t)
	all, err := s.Tree("feature-x", "", true, 0)
	must(t, err)
	b, err := MarshalIndent(all)
	must(t, err)
	for _, secret := range []string{"PAYLOAD-SECRET-1", "RESULT-SECRET-2", "payload", "token", "session"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("tree JSON leaks %q: %s", secret, b)
		}
	}
	var back map[string]any
	must(t, json.Unmarshal(b, &back))
	node := back["nodes"].([]any)[2].(map[string]any)
	for _, k := range []string{"thread", "depth", "state", "assigned_by", "assign_id"} {
		if _, ok := node[k]; !ok {
			t.Errorf("node JSON missing %q: %v", k, node)
		}
	}
	// The tree is read-only: it works on an expired channel.
	c.add(3 * time.Hour)
	if _, err := s.Tree("feature-x", "", false, 0); err != nil {
		t.Fatalf("expired channel: %v", err)
	}
}

// v3Schema is a frozen copy of the schema as shipped before delegation trees.
const v3Schema = `
CREATE TABLE channels (name TEXT PRIMARY KEY, owner TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER, lease_ns INTEGER NOT NULL DEFAULT 120000000000);
CREATE TABLE invites (code_hash TEXT PRIMARY KEY, channel TEXT NOT NULL REFERENCES channels(name), role TEXT NOT NULL, bound_name TEXT, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, used_at INTEGER, used_by TEXT);
CREATE TABLE agents (channel TEXT NOT NULL REFERENCES channels(name), name TEXT NOT NULL, role TEXT NOT NULL, card TEXT NOT NULL, token_hash TEXT NOT NULL, joined_at INTEGER NOT NULL, cursor INTEGER NOT NULL DEFAULT 0, last_seen INTEGER, PRIMARY KEY (channel, name));
CREATE TABLE messages (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, channel TEXT NOT NULL REFERENCES channels(name), sender TEXT NOT NULL, recipient TEXT NOT NULL, thread TEXT, type TEXT NOT NULL, causation_id TEXT, idempotency_key TEXT, ttl TEXT, expires_at INTEGER, payload TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE UNIQUE INDEX messages_idem ON messages(channel, sender, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX messages_channel_seq ON messages(channel, seq);
CREATE TABLE claims (channel TEXT NOT NULL REFERENCES channels(name), thread TEXT NOT NULL, owner TEXT NOT NULL, claimed_at INTEGER NOT NULL, completed_at INTEGER, result_id TEXT, epoch INTEGER NOT NULL DEFAULT 1, released_at INTEGER, prev_owner TEXT, revoked_at INTEGER, revoked_by TEXT, revoke_reason TEXT, confinement TEXT, PRIMARY KEY (channel, thread));
INSERT INTO channels(name, owner, created_at) VALUES ('c', 'boss', 1);
INSERT INTO agents(channel, name, role, card, token_hash, joined_at, cursor, last_seen) VALUES ('c', 'boss', 'supervisor', '{"name":"boss"}', 'h', 1, 7, 9223372036854775807);
INSERT INTO messages(id, channel, sender, recipient, thread, type, payload, created_at) VALUES ('m1', 'c', 'agent:boss', 'role:worker', 'T', 'task.assign', '{}', 1);
INSERT INTO claims(channel, thread, owner, claimed_at, epoch) VALUES ('c', 'T', 'agent:boss', 1, 2);
PRAGMA user_version = 3;
`

func writeV3Store(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	must(t, os.MkdirAll(Dir(dir), 0o700))
	db, err := sql.Open("sqlite", filepath.Join(Dir(dir), "bus.db"))
	must(t, err)
	if _, err := db.Exec(v3Schema + extra); err != nil {
		t.Fatal(err)
	}
	must(t, db.Close())
	return dir
}

func userVersion(t *testing.T, s *Store) int {
	t.Helper()
	var v int
	must(t, s.db.QueryRow(`PRAGMA user_version`).Scan(&v))
	return v
}

func TestMigrateV3ToV4(t *testing.T) {
	dir := writeV3Store(t, "")
	s, err := Open(dir)
	must(t, err)
	defer s.Close()
	if v := userVersion(t, s); v != 4 {
		t.Fatalf("user_version %d", v)
	}
	var n int
	must(t, s.db.QueryRow(`SELECT COUNT(*) FROM task_links`).Scan(&n))
	if n != 0 {
		t.Fatalf("migrated store has %d links", n)
	}
	msgs, err := s.Log("c", "", 10)
	must(t, err)
	if len(msgs) != 1 || msgs[0].Parent != "" || msgs[0].Thread != "T" {
		t.Fatalf("messages after migration: %+v", msgs)
	}
	c, err := s.Claim("c", "T")
	must(t, err)
	if c.Epoch != 2 {
		t.Fatalf("claim after migration: %+v", c)
	}
	// Existing threads are roots, and the migrated store accepts parented sends.
	tr, err := s.Tree("c", "", true, 0)
	must(t, err)
	if len(tr.Nodes) != 1 || tr.Nodes[0].Thread != "T" || tr.Nodes[0].Parent != "" {
		t.Fatalf("tree: %+v", tr.Nodes)
	}
	boss := Member{Channel: "c", Name: "boss", Role: RoleSupervisor}
	mustAssign(t, s, boss, "T/child", "T")
	// Reopening a v4 store is a no-op.
	must(t, s.Close())
	s2, err := Open(dir)
	must(t, err)
	must(t, s2.Close())
}

// A failing step rolls back and leaves the store at v3, never half migrated.
func TestMigrateV3FailureLeavesV3(t *testing.T) {
	dir := writeV3Store(t, `ALTER TABLE messages ADD COLUMN parent TEXT;`)
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("migration over a conflicting column succeeded")
	}
	db, err := sql.Open("sqlite", filepath.Join(Dir(dir), "bus.db"))
	must(t, err)
	defer db.Close()
	var v, links int
	must(t, db.QueryRow(`PRAGMA user_version`).Scan(&v))
	must(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'task_links'`).Scan(&links))
	if v != 3 || links != 0 {
		t.Fatalf("after failed migration: version %d, task_links tables %d", v, links)
	}
}

// Keep the fresh schema and the migration producing the same shape.
func TestFreshAndMigratedSchemasMatch(t *testing.T) {
	fresh, err := Open(t.TempDir())
	must(t, err)
	defer fresh.Close()
	mig, err := Open(writeV3Store(t, ""))
	must(t, err)
	defer mig.Close()
	cols := func(s *Store, table string) string {
		rows, err := s.db.Query(`SELECT name FROM pragma_table_info('` + table + `') ORDER BY name`)
		must(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n string
			must(t, rows.Scan(&n))
			out = append(out, n)
		}
		return strings.Join(out, ",")
	}
	for _, tb := range []string{"messages", "task_links", "claims"} {
		if a, b := cols(fresh, tb), cols(mig, tb); a != b {
			t.Errorf("%s: fresh %q, migrated %q", tb, a, b)
		}
	}
}
