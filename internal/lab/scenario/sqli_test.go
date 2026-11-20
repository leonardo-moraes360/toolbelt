package scenario

import "testing"

func TestSQLiScenarioReproduces(t *testing.T) {
	s, err := For("sql-injection")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	out := s.Run()
	if out.Attack != AttackReproduced {
		t.Errorf("attack = %q, want reproduced", out.Attack)
	}
	if out.Observable == "" {
		t.Error("observable must be non-empty for the blue side to inspect")
	}
}

func TestVulnerableLoginBypass(t *testing.T) {
	users := map[string]string{"admin": "s3cr3t"}
	// Tautology injection authenticates despite a wrong password.
	if got := vulnerableLogin(users, `admin' OR '1'='1`, "wrong"); got == 0 {
		t.Error("injection should have matched at least one row")
	}
	// A plain wrong password must not authenticate.
	if got := vulnerableLogin(users, "admin", "wrong"); got != 0 {
		t.Errorf("wrong password matched %d row(s), want 0", got)
	}
	// The right password authenticates exactly the one user.
	if got := vulnerableLogin(users, "admin", "s3cr3t"); got != 1 {
		t.Errorf("valid login matched %d row(s), want 1", got)
	}
}

func TestEvalPredicate(t *testing.T) {
	cases := []struct {
		where      string
		user, pass string
		want       bool
	}{
		{"user='admin' AND pass='s3cr3t'", "admin", "s3cr3t", true},
		{"user='admin' AND pass='s3cr3t'", "admin", "wrong", false},
		{"user='admin' OR '1'='1'", "alice", "pw", true},
		{"'1'='2'", "admin", "s3cr3t", false},
	}
	for _, c := range cases {
		if got := evalPredicate(c.where, c.user, c.pass); got != c.want {
			t.Errorf("evalPredicate(%q, %q, %q) = %v, want %v", c.where, c.user, c.pass, got, c.want)
		}
	}
}

func TestClassesRegistered(t *testing.T) {
	cs := Classes()
	found := false
	for _, c := range cs {
		if c == "sql-injection" {
			found = true
		}
	}
	if !found {
		t.Errorf("sql-injection not registered; have %v", cs)
	}
}
