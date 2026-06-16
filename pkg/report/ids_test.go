package report

import "testing"

func TestAssignShortIDs_FindingsGetUniqueHandles(t *testing.T) {
	r := &Report{Findings: []Finding{
		{Fingerprint: "aaaa1111", Message: "a"},
		{Fingerprint: "bbbb2222", Message: "b"},
		{Fingerprint: "cccc3333", Message: "c"},
	}}
	AssignShortIDs(r)

	seen := map[string]bool{}
	for _, f := range r.Findings {
		if len(f.ID) != len("F-")+minPrefix {
			t.Errorf("ID %q: want %d-char prefix, got len %d", f.ID, minPrefix, len(f.ID)-2)
		}
		if f.ID[:2] != "F-" {
			t.Errorf("ID %q: want F- prefix", f.ID)
		}
		if seen[f.ID] {
			t.Errorf("duplicate ID %q", f.ID)
		}
		seen[f.ID] = true
	}
}

func TestAssignShortIDs_GrowsPrefixOnCollision(t *testing.T) {
	// First 3 hex chars collide ("abc"); 4th distinguishes.
	r := &Report{Findings: []Finding{
		{Fingerprint: "abc1ffff"},
		{Fingerprint: "abc2ffff"},
	}}
	AssignShortIDs(r)

	if r.Findings[0].ID == r.Findings[1].ID {
		t.Fatalf("collision not resolved: both %q", r.Findings[0].ID)
	}
	if got := len(r.Findings[0].ID) - 2; got != 4 {
		t.Errorf("want prefix grown to 4, got %d (%q)", got, r.Findings[0].ID)
	}
}

func TestAssignShortIDs_SameFingerprintSharesID(t *testing.T) {
	r := &Report{Findings: []Finding{
		{Fingerprint: "dead1234", Line: 10},
		{Fingerprint: "dead1234", Line: 99}, // same defect, different line
	}}
	AssignShortIDs(r)
	if r.Findings[0].ID != r.Findings[1].ID {
		t.Errorf("same fingerprint should share ID: %q vs %q", r.Findings[0].ID, r.Findings[1].ID)
	}
	if r.Findings[0].ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestAssignShortIDs_EmptyFingerprintNoID(t *testing.T) {
	r := &Report{
		Findings: []Finding{{Fingerprint: ""}},
		Tests: []TestResult{
			{Outcome: OutcomePass, Fingerprint: ""},
			{Outcome: OutcomeFail, Fingerprint: "f00d5678"},
		},
	}
	AssignShortIDs(r)
	if r.Findings[0].ID != "" {
		t.Errorf("empty fingerprint should get no ID, got %q", r.Findings[0].ID)
	}
	if r.Tests[0].ID != "" {
		t.Errorf("passing test should get no ID, got %q", r.Tests[0].ID)
	}
	if r.Tests[1].ID == "" || r.Tests[1].ID[:2] != "T-" {
		t.Errorf("failing test should get T- ID, got %q", r.Tests[1].ID)
	}
}

func TestAssignShortIDs_SeparateNamespaces(t *testing.T) {
	// A finding and a test sharing a fingerprint must not collide — they
	// live in independent F-/T- namespaces.
	r := &Report{
		Findings: []Finding{{Fingerprint: "12345678"}},
		Tests:    []TestResult{{Outcome: OutcomeFail, Fingerprint: "12345678"}},
	}
	AssignShortIDs(r)
	if r.Findings[0].ID[:2] != "F-" || r.Tests[0].ID[:2] != "T-" {
		t.Errorf("namespaces crossed: %q / %q", r.Findings[0].ID, r.Tests[0].ID)
	}
}

func TestAssignShortIDs_NilReport(t *testing.T) {
	AssignShortIDs(nil) // must not panic
}

func TestAssignShortIDsStable_ReusesPriorHandle(t *testing.T) {
	// Prior run pinned aaaa1111 to the 4-char handle F-aaaa; even though
	// its natural prefix would be the 3-char F-aaa, pinning preserves the
	// longer handle so the agent's reference stays valid.
	prior := &Prior{Findings: map[string]string{"aaaa1111": "F-aaaa"}}
	r := &Report{Findings: []Finding{
		{Fingerprint: "aaaa1111"},
		{Fingerprint: "bbbb2222"},
	}}
	AssignShortIDsStable(r, prior)
	if r.Findings[0].ID != "F-aaaa" {
		t.Errorf("pinned handle not reused: got %q want F-aaaa", r.Findings[0].ID)
	}
	if r.Findings[1].ID == "F-aaaa" {
		t.Errorf("new finding collided with pinned handle: %q", r.Findings[1].ID)
	}
}

func TestAssignShortIDsStable_RejectsStalePrior(t *testing.T) {
	// Prior handle's hex is not a prefix of the current fingerprint
	// (fingerprint changed) → must not reuse; assign fresh instead.
	prior := &Prior{Findings: map[string]string{"deadbeef": "F-999"}}
	r := &Report{Findings: []Finding{{Fingerprint: "deadbeef"}}}
	AssignShortIDsStable(r, prior)
	if r.Findings[0].ID != "F-dea" {
		t.Errorf("stale prior should be ignored: got %q want F-dea", r.Findings[0].ID)
	}
}

func TestAssignShortIDsStable_MintAvoidsReusedHex(t *testing.T) {
	// Pinned handle F-abc occupies "abc"; a new fingerprint sharing that
	// 3-prefix must grow past it rather than collide.
	prior := &Prior{Findings: map[string]string{"abc00000": "F-abc"}}
	r := &Report{Findings: []Finding{
		{Fingerprint: "abc00000"},
		{Fingerprint: "abc99999"},
	}}
	AssignShortIDsStable(r, prior)
	if r.Findings[0].ID != "F-abc" {
		t.Errorf("pinned reuse failed: %q", r.Findings[0].ID)
	}
	if r.Findings[1].ID == "F-abc" {
		t.Errorf("mint collided with pinned: %q", r.Findings[1].ID)
	}
}
