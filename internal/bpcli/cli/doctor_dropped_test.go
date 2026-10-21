package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckAuditDropped pins the doctor surface of the dropped-audit
// contract (governance.AppendDroppedAuditMarker + AuditLog.persist): an
// absent or empty sidecar is ok; a non-empty sidecar warns with the entry
// count and the last drop's timestamp.
func TestCheckAuditDropped(t *testing.T) {
	t.Run("absent sidecar is ok", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		c := checkAuditDropped()
		if c.Status != statusOK {
			t.Fatalf("status = %q, want ok (no sidecar)", c.Status)
		}
	})
	t.Run("empty sidecar is ok", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		dir := filepath.Join(home, ".kern", "audit")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "dropped.jsonl"), []byte("\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		c := checkAuditDropped()
		if c.Status != statusOK {
			t.Fatalf("status = %q, want ok (empty sidecar)", c.Status)
		}
	})
	t.Run("non-empty sidecar warns with count and last timestamp", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		dir := filepath.Join(home, ".kern", "audit")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		body := `{"ts":"2026-10-08T12:00:00Z","id":"audit-1","action":"tool_call","resource":"kern_search","cause":"disk full"}` + "\n" +
			`{"ts":"2026-10-08T12:05:00Z","id":"audit-2","action":"tool_call","resource":"kern_explore","cause":"disk full"}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "dropped.jsonl"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		c := checkAuditDropped()
		if c.Status != statusWarn {
			t.Fatalf("status = %q, want warn (non-empty sidecar)", c.Status)
		}
		if !strings.Contains(c.Detail, "2 audit entries") {
			t.Fatalf("detail = %q, want the count of dropped entries", c.Detail)
		}
		if !strings.Contains(c.Detail, "12:05:00") {
			t.Fatalf("detail = %q, want the last drop's timestamp", c.Detail)
		}
	})
}
