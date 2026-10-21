// Package guard owns the boundary-guard family: the declarative
// .kern/boundaries.json guardrails (load/verdict). This is a deliberately
// trimmed slice of kern's internal/guard: kern also ships index-backed
// inference and precise boundary checking (InferBoundaries,
// CheckBoundaries/Precise, their resolveCallee/indexDirs machinery, and the
// intel.Violation re-export) that depend on the kern kernel
// (internal/index, internal/intel). The standalone keeps the declarative
// load + verdict core the adapters/kern architecture check consumes.
package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JayveerPrajapati/blueprint/internal/domain"
)

// Boundaries is the declarative guardrail file loaded from .kern/boundaries.json.
type Boundaries struct {
	Description string                `json:"description,omitempty"`
	Rules       []domain.BoundaryRule `json:"rules"`
	// Pure opts into @pure mutability assertions: when true, `kern guard
	// check` (and the MCP guard tool) also validate that every Go
	// function/method whose doc comment contains "@pure" does not mutate
	// anything outside its own stack frame. Off by default; the flag changes
	// nothing when absent.
	Pure bool `json:"pure,omitempty"`
}

// DefaultBoundariesPath returns where the guardrail rules live for a root.
func DefaultBoundariesPath(root string) string {
	return filepath.Join(root, ".kern", "boundaries.json")
}

// LoadBoundaries reads the guardrail rules for root. A missing file is not an
// error — it yields a nil ruleset (nothing to enforce), preserving the
// zero-config experience. A present-but-malformed file IS an error (fail-closed):
// a broken boundaries.json must never silently permit everything.
func LoadBoundaries(root string) (*Boundaries, error) {
	data, err := os.ReadFile(DefaultBoundariesPath(root))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var b Boundaries
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", DefaultBoundariesPath(root), err)
	}
	return &b, nil
}

// Verdict decides whether the fromDir -> toDir dependency edge is permitted
// by the declared rules. An explicit "allow" wins over any "forbid" for the
// same pair; otherwise the first matching "forbid" is a violation. A nil
// return means the edge is permitted.
func Verdict(rules []domain.BoundaryRule, fromDir, toDir string) *domain.BoundaryRule {
	for i := range rules {
		if rules[i].Action == "allow" && DirMatch(rules[i].From, fromDir) && DirMatch(rules[i].To, toDir) {
			return nil
		}
	}
	for i := range rules {
		if rules[i].Action == "forbid" && DirMatch(rules[i].From, fromDir) && DirMatch(rules[i].To, toDir) {
			return &rules[i]
		}
	}
	return nil
}

// DirMatch is the canonical directory-pattern matcher (exact, "pattern/…"
// prefix, or "…/pattern" suffix match). It matches a rule pattern against a
// directory and is shared by the boundary guard layer and consumers in other
// packages that need the same semantics.
func DirMatch(pattern, dir string) bool {
	if pattern == "" {
		return false
	}
	return dir == pattern ||
		strings.HasPrefix(dir, pattern+"/") ||
		strings.HasSuffix(dir, "/"+pattern)
}
