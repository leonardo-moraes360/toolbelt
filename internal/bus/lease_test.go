package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// third joins a second worker.
func third(t *testing.T, s *Store, name string) Member {
	t.Helper()
	inv, err := s.CreateInvite("feature-x", RoleWorker, "", 0)
	must(t, err)
	j, err := s.Join("feature-x", inv.Code, &AgentCard{Name: name}, "")
	must(t, err)
	return j.Member
}

func boardStatus(t *testing.T, s *Store, owner Member) []Message {
	t.Helper()
	msgs, err := s.Recv(context.Background(), owner, RecvOptions{Peek: true, Limit: 500})
	must(t, err)
	var out []Message
	for i := range msgs {
		if msgs[i].Type == TypeTaskStatus && msgs[i].From == WatchdogSender {
			out = append(out, msgs[i])
		}
	}
	return out
}

// AC1/AC2: takeover of a silent owner, A -> B -> A epochs, stale complete.
func TestLeaseTakeoverAndFencing(t *testing.T) {
	s, c, owner, a := setup(t)
	b := third(t, s, "codex-1")
	ca, err := s.ClaimThread(a, "T1")
	must(t, err)
	if ca.Epoch != 1 {
		t.Fatalf("epoch %d", ca.Epoch)
	}
	_, err = s.ClaimThread(b, "T1")
	wantErr(t, err, "lease ends", "alive owner keeps the thread")

	c.add(DefaultLease + time.Second)
	_, err = s.Heartbeat(b) // only b is alive now
	must(t, err)
	cb, err := s.ClaimThread(b, "T1")
	must(t, err)
	if cb.Epoch != 2 || cb.PrevOwner != a.ID() {
		t.Fatalf("takeover: %+v", cb)
	}
	if ev := boardStatus(t, s, owner); len(ev) != 1 || !strings.Contains(string(ev[0].Payload), `"lease_expired"`) {
		t.Fatalf("lease_expired events: %+v", ev)
	}
	_, err = s.Complete(a, "T1", 1, nil)
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale owner complete: %v", err)
	}
	hb, err := s.Heartbeat(a)
	must(t, err)
	if len(hb.Lost) != 1 || hb.Lost[0].NewOwner != b.ID() || len(hb.Held) != 0 {
		t.Fatalf("heartbeat lost: %+v", hb)
	}

	c.add(DefaultLease + time.Second)
	_, err = s.Heartbeat(a)
	must(t, err)
	ca2, err := s.ClaimThread(a, "T1")
	must(t, err)
	if ca2.Epoch != 3 {
		t.Fatalf("A -> B -> A epoch %d", ca2.Epoch)
	}
	_, err = s.Complete(a, "T1", 1, nil)
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("epoch 1 after A -> B -> A: %v", err)
	}
	_, err = s.Complete(a, "T1", 3, json.RawMessage(`{"ok":true}`))
	must(t, err)
}

// AC2 (late complete) and AC3 (complete after a watchdog release).
func TestLeaseLateCompleteAndWatchdog(t *testing.T) {
	s, c, owner, a := setup(t)
	_, err := s.ClaimThread(a, "T1")
	must(t, err)
	_, err = s.ClaimThread(a, "T2")
	must(t, err)
	c.add(DefaultLease + time.Second)
	_, err = s.Complete(a, "T1", 1, nil) // expired, but nobody took it over
	must(t, err)

	c.add(DefaultLease + time.Second)
	dry, err := s.Watchdog("feature-x", true)
	must(t, err)
	if len(dry) != 1 || dry[0].Thread != "T2" {
		t.Fatalf("dry run: %+v", dry)
	}
	if ev := boardStatus(t, s, owner); len(ev) != 0 {
		t.Fatalf("dry run published: %+v", ev)
	}
	got, err := s.Watchdog("feature-x", false)
	must(t, err)
	again, err := s.Watchdog("feature-x", false)
	must(t, err)
	if len(got) != 1 || len(again) != 0 || len(boardStatus(t, s, owner)) != 1 {
		t.Fatalf("watchdog: %+v / %+v", got, again)
	}
	_, err = s.Complete(a, "T2", 1, nil)
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("complete after release: %v", err)
	}
	re, err := s.ClaimThread(a, "T2")
	must(t, err)
	if re.Epoch != 2 || re.PrevOwner != "" {
		t.Fatalf("reclaim after release: %+v", re)
	}
	_, err = s.Complete(a, "T2", 0, nil)
	wantErr(t, err, "epoch is required", "complete without epoch")
}

// AC4: two concurrent takeovers, exactly one wins.
func TestLeaseConcurrentTakeover(t *testing.T) {
	s, c, _, a := setup(t)
	b := third(t, s, "b-agent")
	d := third(t, s, "d-agent")
	_, err := s.ClaimThread(a, "T")
	must(t, err)
	c.add(DefaultLease + time.Second)
	_, _ = s.Heartbeat(b)
	_, _ = s.Heartbeat(d)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, m := range []Member{b, d} {
		wg.Add(1)
		go func(i int, m Member) {
			defer wg.Done()
			_, errs[i] = s.ClaimThread(m, "T")
		}(i, m)
	}
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("exactly one takeover must win: %v / %v", errs[0], errs[1])
	}
}

// AC5: one heartbeat keeps several claims; peeks and expired channels do
// not renew last_seen.
func TestLeaseRenewal(t *testing.T) {
	s, c, owner, a := setup(t)
	for _, th := range []string{"A", "B"} {
		_, err := s.ClaimThread(a, th)
		must(t, err)
	}
	c.add(DefaultLease - time.Second)
	hb, err := s.Heartbeat(a)
	must(t, err)
	if len(hb.Held) != 2 || hb.HeartbeatInterval != (DefaultLease/3).String() {
		t.Fatalf("held: %+v", hb)
	}
	c.add(DefaultLease - time.Second)
	_, err = s.Recv(context.Background(), a, RecvOptions{Peek: true})
	must(t, err)
	st, err := s.Status("feature-x")
	must(t, err)
	for _, cl := range st.Claims {
		if cl.State != "held" {
			t.Fatalf("still held after heartbeat: %+v", st.Claims)
		}
	}
	c.add(2 * time.Second)
	st, _ = s.Status("feature-x")
	if st.Claims[0].State != "expired" {
		t.Fatalf("a peek renewed the lease: %+v", st.Claims)
	}
	_ = owner
}

// AC8: nobody can send as system:watchdog.
func TestWatchdogSenderReserved(t *testing.T) {
	if err := ValidName("agent", "system:watchdog"); err == nil {
		t.Fatal("system:watchdog accepted as an agent name")
	}
}

// AC11: migrating a v1 store keeps data and everyone alive.
func TestMigrateV1(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt", "bus"), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, ".toolbelt", "bus", "bus.db"))
	must(t, err)
	for _, q := range []string{
		`CREATE TABLE channels (name TEXT PRIMARY KEY, owner TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER)`,
		`CREATE TABLE invites (code_hash TEXT PRIMARY KEY, channel TEXT NOT NULL, role TEXT NOT NULL, bound_name TEXT, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, used_at INTEGER, used_by TEXT)`,
		`CREATE TABLE agents (channel TEXT NOT NULL, name TEXT NOT NULL, role TEXT NOT NULL, card TEXT NOT NULL, token_hash TEXT NOT NULL, joined_at INTEGER NOT NULL, cursor INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (channel, name))`,
		`CREATE TABLE messages (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, channel TEXT NOT NULL, sender TEXT NOT NULL, recipient TEXT NOT NULL, thread TEXT, type TEXT NOT NULL, causation_id TEXT, idempotency_key TEXT, ttl TEXT, expires_at INTEGER, payload TEXT NOT NULL, created_at INTEGER NOT NULL)`,
		`CREATE TABLE claims (channel TEXT NOT NULL, thread TEXT NOT NULL, owner TEXT NOT NULL, claimed_at INTEGER NOT NULL, completed_at INTEGER, result_id TEXT, PRIMARY KEY (channel, thread))`,
		`INSERT INTO channels VALUES ('c', 'boss', 1, NULL)`,
		`INSERT INTO agents VALUES ('c', 'boss', 'supervisor', '{"name":"boss"}', 'h', 1, 7)`,
		`INSERT INTO claims VALUES ('c', 'T', 'agent:boss', 1, NULL, NULL)`,
		`INSERT INTO messages (id, channel, sender, recipient, type, payload, created_at) VALUES ('m1', 'c', 'agent:boss', 'topic:board', 'query', '{}', 1)`,
		`PRAGMA user_version = 1`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	must(t, db.Close())
	before := time.Now()
	s, err := Open(dir)
	must(t, err)
	defer s.Close()
	st, err := s.Status("c")
	must(t, err)
	if len(st.Agents) != 1 || !st.Agents[0].Alive || len(st.Claims) != 1 || st.Claims[0].Epoch != 1 || st.Claims[0].State != "held" || st.Lease != DefaultLease.String() {
		t.Fatalf("migrated: %+v", st)
	}
	var cursor int64
	must(t, s.db.QueryRow(`SELECT cursor FROM agents WHERE name = 'boss'`).Scan(&cursor))
	if cursor != 7 {
		t.Fatalf("cursor %d", cursor)
	}
	if ls := st.Agents[0].LastSeen; ls.Before(before.Add(-time.Second)) || ls.After(time.Now()) {
		t.Fatalf("last_seen %v is not the migration time", ls)
	}
	msgs, err := s.Log("c", "", 10)
	must(t, err)
	if len(msgs) != 1 {
		t.Fatalf("messages after migration: %+v", msgs)
	}
	must(t, s.Close())

	db, err = sql.Open("sqlite", filepath.Join(dir, ".toolbelt", "bus", "bus.db"))
	must(t, err)
	_, err = db.Exec(`PRAGMA user_version = 4`)
	must(t, err)
	must(t, db.Close())
	if s3, err := Open(dir); err == nil {
		s3.Close()
		t.Fatal("a v4 store was accepted")
	}
}

func TestChannelLeaseBounds(t *testing.T) {
	s, _ := openTest(t)
	if _, _, err := s.CreateChannelLease("x", "o", 0, 0, 10*time.Second); err == nil {
		t.Fatal("lease below the minimum accepted")
	}
	ch, _, err := s.CreateChannelLease("y", "o", 0, 0, time.Minute)
	must(t, err)
	if ch.Lease != time.Minute {
		t.Fatalf("lease %s", ch.Lease)
	}
}

func aliveOf(t *testing.T, s *Store, name string) AgentLease {
	t.Helper()
	st, err := s.Status("feature-x")
	must(t, err)
	for _, a := range st.Agents {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("agent %s missing", name)
	return AgentLease{}
}

// AC5: send and an advancing recv renew; an empty recv --wait and calls on
// an expired channel do not. R9: extend brings agents back not alive.
func TestLeaseRenewalSources(t *testing.T) {
	s, c, owner, a := setup(t)
	c.add(time.Minute)
	_, err := s.Recv(context.Background(), a, RecvOptions{Wait: time.Millisecond})
	must(t, err)
	if got := aliveOf(t, s, a.Name).LastSeen; !got.Before(c.now()) {
		t.Fatal("an empty recv --wait renewed last_seen")
	}
	_, err = s.Send(owner, &SendInput{To: a.ID(), Type: TypeQuery, Payload: json.RawMessage(`{}`)})
	must(t, err)
	if !aliveOf(t, s, owner.Name).LastSeen.Equal(c.now()) {
		t.Fatal("send did not renew last_seen")
	}
	c.add(time.Second)
	msgs, err := s.Recv(context.Background(), a, RecvOptions{})
	must(t, err)
	if len(msgs) == 0 || !aliveOf(t, s, a.Name).LastSeen.Equal(c.now()) {
		t.Fatal("an advancing recv did not renew last_seen")
	}

	c.add(3 * time.Hour) // past the channel --ttl
	if _, err := s.Heartbeat(a); !errors.Is(err, ErrChannelExpired) {
		t.Fatalf("heartbeat on an expired channel: %v", err)
	}
	if _, err := s.Watchdog("feature-x", false); !errors.Is(err, ErrChannelExpired) {
		t.Fatalf("watchdog on an expired channel: %v", err)
	}
	_, err = s.ExtendChannel("feature-x", time.Hour)
	must(t, err)
	if aliveOf(t, s, a.Name).Alive {
		t.Fatal("extend revived an agent that did not heartbeat")
	}
}

// AC7: the watchdog leaves claims of alive owners alone.
func TestWatchdogSparesAliveOwners(t *testing.T) {
	s, c, _, a := setup(t)
	b := third(t, s, "b-agent")
	_, err := s.ClaimThread(a, "dead")
	must(t, err)
	c.add(DefaultLease + time.Second)
	_, err = s.ClaimThread(b, "live")
	must(t, err)
	got, err := s.Watchdog("feature-x", false)
	must(t, err)
	if len(got) != 1 || got[0].Thread != "dead" {
		t.Fatalf("watchdog released %+v", got)
	}
}
