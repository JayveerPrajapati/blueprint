package secscan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JayveerPrajapati/blueprint/internal/blueprint/domain"
)

// Check implements service.Check using the in-house secscan core with NO
// external binary. It is the final tier of the secret gate: it runs when
// neither the gitleaks binary nor the kern binary is available (degraded
// mode), so the secrets gate still produces real detections — never a silent
// pass.
type Check struct {
	allowlist Allowlist
}

// Allowlist decides whether a finding in a given file should be suppressed.
// The default allowlist suppresses test fixtures (testdata/ dirs, _test.go
// files) — the same semantics as the kern SecretCheck's DefaultAllowlist, so
// fixture behavior can never drift between the tiers.
type Allowlist interface {
	IsAllowed(filePath string) bool
}

// DefaultAllowlist suppresses findings in test fixtures: files under a
// testdata/ directory, files with a _test.go suffix, and files with a
// .test.{js,ts,py,rb} suffix.
type DefaultAllowlist struct{}

// IsAllowed returns true for test-fixture paths that should not be flagged.
func (DefaultAllowlist) IsAllowed(path string) bool {
	lower := strings.ToLower(path)
	// testdata/ directory convention (Go, and common across languages).
	if strings.Contains(lower, "/testdata/") || strings.HasPrefix(lower, "testdata/") {
		return true
	}
	// Go test files.
	if strings.HasSuffix(lower, "_test.go") {
		return true
	}
	// JS/TS/Python/Ruby test files.
	for _, suffix := range []string{".test.js", ".test.jsx", ".test.ts", ".test.tsx", ".spec.js", ".spec.ts", "_spec.rb"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// NewCheck constructs an in-house secscan-backed secret check with the default
// allowlist.
func NewCheck() *Check {
	return &Check{allowlist: DefaultAllowlist{}}
}

// NewCheckWithAllowlist constructs a Check with a custom allowlist.
func NewCheckWithAllowlist(allowlist Allowlist) *Check {
	return &Check{allowlist: allowlist}
}

// Name returns the stable check identifier used for policy routing. The
// "secret:" prefix routes this check to the secret category
// (policy.categoryFromCheck splits on the first colon).
func (c *Check) Name() string { return "secret:secscan" }

// Run scans the changed files for secrets with the in-house core. Proposed
// content (Content != "") is scanned directly; on-disk files are read from
// the repository. Findings are redacted (the raw snippet NEVER propagates —
// same contract as the gitleaks and kern tiers). The new-change principle
// holds: when the diff hunks for a file are known, only secrets on lines the
// change actually ADDS are reported.
func (c *Check) Run(ctx context.Context, req domain.ChangeRequest) (domain.CheckResult, error) {
	if req.RepositoryRoot == "" {
		return domain.CheckResult{Name: c.Name(), Status: domain.StatusError, Error: "repository root required"}, nil
	}

	// No changed files => nothing to scan: PASS without invoking the scanner.
	if len(req.Files) == 0 {
		return domain.CheckResult{Name: c.Name(), Status: domain.StatusPass}, nil
	}

	// addedLinesByFile maps a changed file to the exact NEW-file line numbers
	// its diff hunks introduce (domain.FileChange.Added). When present,
	// findings are only reported for those lines: a secret that pre-dates the
	// change in a modified file is not part of this change and must not be
	// attributed to it. Files without hunk data (Content-provided proposals,
	// CI diffs) keep the whole-content semantics.
	addedLinesByFile := make(map[string]map[int]bool)
	var targets []ScanTarget
	for _, fc := range req.Files {
		if fc.Op == domain.OpDelete {
			continue
		}
		// Confine changed paths: reject anything that would escape the
		// repository via ".." or an absolute path (same pattern as the
		// gitleaks and kern tiers).
		rel := filepath.Clean(fc.Path)
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return domain.CheckResult{Name: c.Name(), Status: domain.StatusError, Error: fmt.Sprintf("invalid path in change request: %q", fc.Path)}, nil
		}

		if len(fc.Added) > 0 {
			lines := make(map[int]bool, len(fc.Added))
			for _, ls := range fc.Added {
				if n, err := strconv.Atoi(ls); err == nil {
					lines[n] = true
				}
			}
			addedLinesByFile[normalizePath(fc.Path)] = lines
		}

		content := fc.Content
		if content == "" {
			b, err := os.ReadFile(filepath.Join(req.RepositoryRoot, fc.Path))
			if err != nil {
				// An unreadable changed file must not silently vanish from
				// the scan (fail closed, like gitleaks): surface it as an
				// error rather than a misleading clean verdict.
				return domain.CheckResult{Name: c.Name(), Status: domain.StatusError, Error: fmt.Sprintf("secscan: read %s: %v", fc.Path, err)}, nil
			}
			content = string(b)
		}

		targets = append(targets, ScanTarget{Rel: normalizePath(fc.Path), Src: []byte(content)})
	}

	var findings []domain.Finding
	for _, f := range ScanFiles(targets) {
		file := normalizePath(f.File)

		// when the diff hunks for this file are known, only report secrets
		// on lines the change actually ADDS. A pre-existing secret on an
		// unchanged line of a modified file is not this change's finding.
		if added, ok := addedLinesByFile[file]; ok && !added[f.Line] {
			continue
		}

		// Suppress allowlisted test fixtures (testdata/ dirs, _test.go files).
		if c.allowlist != nil && c.allowlist.IsAllowed(file) {
			continue
		}

		findings = append(findings, c.findingFromScan(f))
	}

	findings = dedupeFindings(findings)

	status := domain.StatusPass
	for _, f := range findings {
		switch f.Severity {
		case domain.SeverityBlock:
			status = domain.StatusBlock
		case domain.SeverityWarn:
			if status == domain.StatusPass {
				status = domain.StatusWarn
			}
		}
	}
	return domain.CheckResult{Name: c.Name(), Status: status, Findings: findings}, nil
}

// findingFromScan maps one in-house finding into a domain.Finding. Redaction
// rule is absolute: the raw snippet from the scanner is NEVER propagated
// (spec: "never echo the secret itself into agent feedback").
func (c *Check) findingFromScan(f Finding) domain.Finding {
	return domain.Finding{
		RuleID:       "secret:secscan:" + f.Rule,
		Severity:     severityFromScan(f.Severity),
		Category:     domain.CategorySecret,
		File:         f.File,
		Line:         f.Line,
		Message:      f.Message,
		Explanation:  fmt.Sprintf("In-house secret scanner detected %s. The value has been redacted and must not be committed. Move the credential to runtime secret storage (env var, vault, or secret manager).", f.Message),
		SuggestedFix: "Move the credential to runtime secret storage (environment variable, vault, or secret manager).",
		Redacted:     true, // CRITICAL: snippet never propagates.
		RuleVersion:  "1",
		Confidence:   0.95,
		Scope:        "file",
		Evidence: []domain.Evidence{{
			Kind:        "pattern-match",
			Description: fmt.Sprintf("rule: %s", f.Rule),
			Location:    fmt.Sprintf("%s:%d", f.File, f.Line),
		}},
	}
}

// severityFromScan maps the in-house severities onto domain severities.
// Unknown severities map to Info so they can never escalate a result.
func severityFromScan(s string) domain.Severity {
	switch s {
	case "error":
		return domain.SeverityBlock
	case "warning":
		return domain.SeverityWarn
	case "info":
		return domain.SeverityInfo
	default:
		return domain.SeverityInfo
	}
}

// dedupeFindings removes duplicate findings with the same normalized file and
// line, keeping the FIRST occurrence.
func dedupeFindings(findings []domain.Finding) []domain.Finding {
	if len(findings) < 2 {
		return findings
	}
	seen := make(map[string]bool, len(findings))
	out := make([]domain.Finding, 0, len(findings))
	for _, f := range findings {
		key := normalizePath(f.File) + ":" + strconv.Itoa(f.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

// normalizePath normalizes a file path to forward-slash, relative form for
// comparison. It strips leading "./" and converts OS-specific separators.
func normalizePath(p string) string {
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return strings.TrimLeft(p, "/")
}