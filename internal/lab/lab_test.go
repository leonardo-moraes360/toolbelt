package lab

import (
	"testing"
	"time"
)

func TestParseValidManifest(t *testing.T) {
	src := []byte(`name: sqli
class: sql-injection
tier: sandbox
network:
  egress: deny
nodes:
  target:
    build: builtin:sql-injection
    expose: [8080]
limits:
  timeout: "5m"
`)
	l, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if l.Name != "sqli" || l.Class != "sql-injection" || l.Tier != TierSandbox {
		t.Errorf("unexpected manifest: %+v", l)
	}
	if d, _ := l.Limits.TimeoutDuration(); d != 5*time.Minute {
		t.Errorf("timeout = %v, want 5m", d)
	}
}

func TestParseDefaults(t *testing.T) {
	l, err := Parse([]byte("name: x\nclass: sql-injection\nnodes:\n  target:\n    build: b\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if l.Tier != TierAuto {
		t.Errorf("tier default = %q, want auto", l.Tier)
	}
	if l.Network.Egress != EgressDeny {
		t.Errorf("egress default = %q, want deny", l.Network.Egress)
	}
	if d, _ := l.Limits.TimeoutDuration(); d != 10*time.Minute {
		t.Errorf("timeout default = %v, want 10m", d)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"bad name":    "name: Bad_Name\nclass: sql-injection\nnodes:\n  target:\n    build: b\n",
		"no class":    "name: x\nnodes:\n  target:\n    build: b\n",
		"bad tier":    "name: x\nclass: c\ntier: docker\nnodes:\n  target:\n    build: b\n",
		"bad egress":  "name: x\nclass: c\nnetwork:\n  egress: allow-all\nnodes:\n  target:\n    build: b\n",
		"no target":   "name: x\nclass: c\nnodes: {}\n",
		"neg pids":    "name: x\nclass: c\nnodes:\n  target:\n    build: b\nlimits:\n  pids: -1\n",
		"bad timeout": "name: x\nclass: c\nnodes:\n  target:\n    build: b\nlimits:\n  timeout: nope\n",
	}
	for name, src := range cases {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
