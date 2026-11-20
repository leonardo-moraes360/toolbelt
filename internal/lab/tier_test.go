package lab

import "testing"

func TestResolveTier(t *testing.T) {
	single := &Lab{Name: "a", Class: "c", Tier: TierAuto, Nodes: Nodes{Target: Node{Build: "b"}}}
	topo := &Lab{Name: "a", Class: "c", Tier: TierAuto, Network: Network{Internal: true}, Nodes: Nodes{Target: Node{Build: "b"}}}
	redBlue := &Lab{Name: "a", Class: "c", Tier: TierAuto, Nodes: Nodes{Target: Node{Build: "b"}, Blue: Node{Playbook: "blue-sqli"}}}

	cases := []struct {
		name     string
		lab      *Lab
		override string
		want     Tier
		wantErr  bool
	}{
		{"auto single-step", single, "", TierSandbox, false},
		{"auto internal net", topo, "", TierPodman, false},
		{"auto red/blue nodes", redBlue, "", TierPodman, false},
		{"override sandbox", topo, "sandbox", TierSandbox, false},
		{"override podman", single, "podman", TierPodman, false},
		{"bad override", single, "docker", "", true},
	}
	for _, c := range cases {
		got, err := ResolveTier(c.lab, c.override)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: tier = %q, want %q", c.name, got, c.want)
		}
	}
}
