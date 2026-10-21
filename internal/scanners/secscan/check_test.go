package secscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JayveerPrajapati/blueprint/internal/blueprint/domain"
)

func checkReq(root string, files ...domain.FileChange) domain.ChangeRequest {
	return domain.ChangeRequest{RepositoryRoot: root, Files: files}
}

func TestCheckEmptyRequestErrors(t *testing.T) {
	res, err := NewCheck().Run(nil, domain.ChangeRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusError {
		t.Fatalf("expected error status without repo root, got %v", res.Status)
	}
}

func TestCheckNoFilesPasses(t *testing.T) {
	res, err := NewCheck().Run(nil, checkReq("/tmp/none"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusPass {
		t.Fatalf("expected pass for empty change, got %v", res.Status)
	}
	if res.Name != "secret:secscan" {
		t.Fatalf("unexpected check name: %s", res.Name)
	}
}

func TestCheckBlocksOnSecret(t *testing.T) {
	root := t.TempDir()
	content := "package main\n\nconst key = \"sk-abcdefghijklmnopqrstuvwxyz1234567890\"\n"
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "auth.go", Content: content}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusBlock {
		t.Fatalf("expected block for hardcoded secret, got %v (%+v)", res.Status, res.Findings)
	}
	f := res.Findings[0]
	if f.RuleID != "secret:secscan:hardcoded-secret" {
		t.Fatalf("unexpected rule id: %s", f.RuleID)
	}
	if !f.Redacted {
		t.Fatal("finding must be redacted")
	}
	// The raw secret must never propagate.
	for _, fd := range res.Findings {
		if strings.Contains(fd.Message+fd.Explanation, "sk-abcdefghijklmnopqrstuvwxyz1234567890") {
			t.Fatalf("secret leaked into finding: %+v", fd)
		}
	}
}

func TestCheckReadsDiskFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "creds.dat"), []byte("PASSWORD=hunter2secret123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "creds.dat", Op: domain.OpWrite}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusBlock {
		t.Fatalf("expected block for disk-file secret, got %v", res.Status)
	}
}

func TestCheckAddedLinesFilter(t *testing.T) {
	root := t.TempDir()
	// The secret sits on line 1; only line 2 is ADDED by the change. The
	// pre-existing secret must not be attributed to this change.
	content := "const key = \"sk-abcdefghijklmnopqrstuvwxyz1234567890\"\n// new line\n"
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "auth.go", Content: content, Added: []string{"2"}}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusPass {
		t.Fatalf("pre-existing secret on un-added line must not block, got %v (%+v)", res.Status, res.Findings)
	}
	// When the secret line IS added, it blocks.
	res2, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "auth.go", Content: content, Added: []string{"1", "2"}}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res2.Status != domain.StatusBlock {
		t.Fatalf("secret on added line must block, got %v", res2.Status)
	}
}

func TestCheckSuppressesTestFixtures(t *testing.T) {
	root := t.TempDir()
	content := "const key = \"sk-abcdefghijklmnopqrstuvwxyz1234567890\"\n"
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "testdata/creds.go", Content: content}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusPass {
		t.Fatalf("testdata fixture must be suppressed, got %v", res.Status)
	}
}

func TestCheckRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "../escape.go", Content: "x"}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusError {
		t.Fatalf("path escape must error, got %v", res.Status)
	}
}

func TestCheckWarningSeverity(t *testing.T) {
	root := t.TempDir()
	// unsafe-exec is a warning-severity rule.
	content := "func run(bin, arg string) {\n\tout, _ := exec.Command(bin, arg).Output()\n\t_ = out\n}\n"
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "exec.go", Content: content}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusWarn {
		t.Fatalf("expected warn for warning-severity finding, got %v (%+v)", res.Status, res.Findings)
	}
	for _, f := range res.Findings {
		if f.RuleID != "secret:secscan:unsafe-exec" {
			t.Fatalf("unexpected rule: %s", f.RuleID)
		}
	}
}

func TestCheckDedupesByFileLine(t *testing.T) {
	root := t.TempDir()
	// One line holding two distinct secrets (GITHUB + TOKEN) stays two findings.
	content := "key = \"ghp_abcdefghijklmnopqrstuvwxyz1234567890\" token = \"0123456789abcdef0123456789abcdef\"\n"
	res, err := NewCheck().Run(nil, checkReq(root,
		domain.FileChange{Path: "creds.go", Content: content}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domain.StatusBlock {
		t.Fatalf("expected block, got %v", res.Status)
	}
	seen := map[string]bool{}
	for _, f := range res.Findings {
		k := f.File + ":" + f.Message
		if seen[k] {
			t.Fatalf("duplicate finding: %+v", f)
		}
		seen[k] = true
	}
}

func TestDefaultAllowlist(t *testing.T) {
	a := DefaultAllowlist{}
	for _, allowed := range []string{"testdata/x.go", "a_test.go", "x.test.js", "x.spec.ts", "x_spec.rb", "y/testdata/z.go"} {
		if !a.IsAllowed(allowed) {
			t.Errorf("expected %q allowed", allowed)
		}
	}
	for _, denied := range []string{"auth.go", "main.go", "x.spec.txt"} {
		if a.IsAllowed(denied) {
			t.Errorf("expected %q not allowed", denied)
		}
	}
}
