package guard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JayveerPrajapati/blueprint/internal/domain"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestVerdictOrderInvariance locks in the allow-dominates semantics: for the
// same (from, to) pair the verdict must be identical regardless of rule slice
// order. forbid-then-allow and allow-then-forbid must both resolve to ALLOW.
func TestVerdictOrderInvariance(t *testing.T) {
	forbidThenAllow := []domain.BoundaryRule{
		{From: "web", To: "db", Action: "forbid"},
		{From: "web", To: "db", Action: "allow"},
	}
	allowThenForbid := []domain.BoundaryRule{
		{From: "web", To: "db", Action: "allow"},
		{From: "web", To: "db", Action: "forbid"},
	}
	if got := Verdict(forbidThenAllow, "web", "db"); got != nil {
		t.Errorf("forbid-then-allow: expected ALLOW (nil), got %+v", got)
	}
	if got := Verdict(allowThenForbid, "web", "db"); got != nil {
		t.Errorf("allow-then-forbid: expected ALLOW (nil), got %+v", got)
	}
}

// TestVerdictForbidWhenNoAllow verifies a lone forbid rule rejects the pair.
func TestVerdictForbidWhenNoAllow(t *testing.T) {
	rules := []domain.BoundaryRule{{From: "web", To: "db", Action: "forbid"}}
	got := Verdict(rules, "web", "db")
	if got == nil {
		t.Fatal("lone forbid rule: expected FORBID, got nil (permitted)")
	}
	if got.From != "web" || got.To != "db" || got.Action != "forbid" {
		t.Errorf("wrong rule returned: %+v", got)
	}
}

// TestVerdictDefaultPermit verifies unconfigured (from, to) pairs remain
// permitted (default-permit), unchanged by the fix.
func TestVerdictDefaultPermit(t *testing.T) {
	rules := []domain.BoundaryRule{{From: "web", To: "db", Action: "forbid"}}
	if got := Verdict(rules, "api", "db"); got != nil {
		t.Errorf("unconfigured pair should default to permitted, got %+v", got)
	}
}

// TestLoadBoundariesMissingFile: an absent .kern/boundaries.json is the
// acceptable zero-config state — nil error AND nil ruleset, never a hard
// failure. The absence is surfaced by a warning log here and as a
// "boundaries-not-configured" skip by CheckBoundariesPrecise, so the check
// layer never turns it into a silent PASS.
func TestLoadBoundariesMissingFile(t *testing.T) {
	dir := writeTree(t, map[string]string{})
	b, err := LoadBoundaries(dir)
	if err != nil {
		t.Fatalf("missing boundaries file must not be an error, got %v", err)
	}
	if b != nil {
		t.Errorf("missing boundaries file should yield a nil ruleset, got %+v", b)
	}
}

// TestLoadBoundariesMalformedFile: a present but invalid boundaries.json must
// fail closed with an error — never silently permit everything.
func TestLoadBoundariesMalformedFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		".kern/boundaries.json": `{"rules": [`,
	})
	if b, err := LoadBoundaries(dir); err == nil {
		t.Fatalf("malformed boundaries file must return an error, got ruleset %+v", b)
	}
}

// TestLoadBoundariesWellFormed: a valid file loads its rules without error.
func TestLoadBoundariesWellFormed(t *testing.T) {
	dir := writeTree(t, map[string]string{
		".kern/boundaries.json": `{"description":"d","rules":[{"from":"web","to":"db","action":"forbid"}]}`,
	})
	b, err := LoadBoundaries(dir)
	if err != nil {
		t.Fatalf("well-formed boundaries file must load: %v", err)
	}
	if b == nil || len(b.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %+v", b)
	}
	if b.Rules[0].Action != "forbid" || b.Rules[0].From != "web" || b.Rules[0].To != "db" {
		t.Errorf("rule decoded incorrectly: %+v", b.Rules[0])
	}
}

// TestLoadBoundariesEmptyRules: an explicitly empty rule set ("rules": []) is
// valid configuration — no error, nothing to enforce.
func TestLoadBoundariesEmptyRules(t *testing.T) {
	dir := writeTree(t, map[string]string{
		".kern/boundaries.json": `{"rules": []}`,
	})
	b, err := LoadBoundaries(dir)
	if err != nil {
		t.Fatalf("empty rules file must not be an error, got %v", err)
	}
	if b == nil || len(b.Rules) != 0 {
		t.Fatalf("expected empty ruleset, got %+v", b)
	}
}
