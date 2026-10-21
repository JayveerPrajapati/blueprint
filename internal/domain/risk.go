// Package domain carries the risk model shared by the governance audit log.
// This is a deliberately trimmed slice of kern's enterprise domain package:
// kern defines the full Approval/Policy/Precheck model here (internal/domain,
// ~2.4k LOC, depending on internal/index and internal/secscan); the standalone
// keeps only the Risk type the audit trail needs. The import path stays
// github.com/JayveerPrajapati/blueprint/internal/domain so the ported
// governance audit.go stays byte-identical modulo the module path.
package domain

// RiskLevel discriminates the severity of a Risk.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// Risk is a risk assessment for a proposed change.
type Risk struct {
	Level      RiskLevel // LOW, MEDIUM, HIGH, CRITICAL
	Factors    []string  // what contributes to the risk
	Score      float64   // 0.0-1.0
	Mitigation string

	// Blocked is true when the governing firewall denied the change outright
	// (unknown agent, missing permission, always-blocked action, or an
	// approval gate that is still pending). When set, the change must not
	// proceed.
	Blocked bool
	// ApprovalRequired is true when the change risk is HIGH/CRITICAL and the
	// governance model requires human approval before the change may proceed.
	ApprovalRequired bool
}
