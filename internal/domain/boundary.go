package domain

// BoundaryRule declares one allowed or forbidden dependency edge between two
// package/directory patterns. Action is "forbid" (a violation) or "allow" (an
// explicit exemption that overrides forbids for the same pair). Ported from
// kern's internal/domain/boundary.go (the trimmed domain carries only the
// types the ported packages consume — here the boundary guard).
type BoundaryRule struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Action string `json:"action"`
}
