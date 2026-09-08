// Package parsers converts the raw output of individual security scanners
// (gitleaks, trivy, ...) into a single shared representation, Finding, so the
// rest of PipelineGuard can consolidate results without caring which tool
// produced them.
package parsers

// Finding is the normalized representation of a single security issue reported
// by any scanner. Every parser in this package maps its tool-specific output
// onto this struct.
type Finding struct {
	// Tool is the name of the scanner that produced the finding, e.g. "gitleaks".
	Tool string

	// Severity is the normalized severity level. One of:
	// CRITICAL | HIGH | MEDIUM | LOW | INFO.
	Severity string

	// File is the path (relative to the scanned repo) where the issue was found.
	File string

	// Line is the 1-based line number the issue starts at. 0 means "unknown".
	Line int

	// Rule is the tool-specific identifier of the rule that fired, e.g. the
	// gitleaks RuleID "generic-api-key".
	Rule string

	// Message is a short human-readable description of the issue.
	Message string
}
