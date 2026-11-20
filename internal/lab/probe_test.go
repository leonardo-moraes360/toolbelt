package lab

import "testing"

// TestProbePodmanFailClosed asserts the podman tier is fail-closed: when it is
// unavailable, ProbePodman reports unavailable with a non-empty remediation and
// never claims availability (A1). CI has no rootless Podman, so the real probe
// exercises the absent path here.
func TestProbePodmanFailClosed(t *testing.T) {
	a := ProbePodman()
	if !a.Available && a.Remediation == "" {
		t.Error("unavailable podman tier must carry a remediation")
	}
	if a.Tier != TierPodman {
		t.Errorf("tier = %q, want podman", a.Tier)
	}
}

func TestProbePodmanOverride(t *testing.T) {
	restore := podmanProbe
	defer func() { podmanProbe = restore }()

	podmanProbe = func() Availability { return Availability{Tier: TierPodman, Available: true} }
	if a := ProbePodman(); !a.Available {
		t.Error("override should report available")
	}

	podmanProbe = func() Availability {
		return Availability{Tier: TierPodman, Remediation: "nope"}
	}
	if a := ProbePodman(); a.Available || a.Remediation != "nope" {
		t.Errorf("override should report unavailable with remediation, got %+v", a)
	}
}
