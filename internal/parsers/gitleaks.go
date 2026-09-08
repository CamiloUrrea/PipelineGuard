package parsers

import (
	"encoding/json"
	"fmt"
)

// gitleaksSeverity is the severity assigned to every gitleaks finding.
// Per ARCHITECTURE.md, any secret leaked in the repository is treated as
// CRITICAL by default; gitleaks does not emit its own severity field.
const gitleaksSeverity = "CRITICAL"

// gitleaksRawFinding mirrors a single object of the JSON array produced by
// `gitleaks detect --report-format json` (gitleaks v8.x). Only the fields
// PipelineGuard needs are declared; the rest of the JSON is ignored.
type gitleaksRawFinding struct {
	Description string `json:"Description"`
	StartLine   int    `json:"StartLine"`
	File        string `json:"File"`
	RuleID      string `json:"RuleID"`
}

// ParseGitleaks takes the raw JSON emitted by
// `gitleaks detect --report-format json` and returns the normalized findings.
//
// The gitleaks report is a JSON array (it is `[]` when nothing is found).
// Malformed JSON produces an error rather than a panic.
//
// Field mapping (gitleaks -> Finding):
//
//	Tool     = "gitleaks" (constant)
//	Severity = "CRITICAL"  (constant, see gitleaksSeverity)
//	File     = File
//	Line     = StartLine
//	Rule     = RuleID
//	Message  = Description
func ParseGitleaks(raw []byte) ([]Finding, error) {
	var rawFindings []gitleaksRawFinding
	if err := json.Unmarshal(raw, &rawFindings); err != nil {
		return nil, fmt.Errorf("parsers: parsing gitleaks report: %w", err)
	}

	findings := make([]Finding, 0, len(rawFindings))
	for _, rf := range rawFindings {
		findings = append(findings, Finding{
			Tool:     "gitleaks",
			Severity: gitleaksSeverity,
			File:     rf.File,
			Line:     rf.StartLine,
			Rule:     rf.RuleID,
			Message:  rf.Description,
		})
	}

	return findings, nil
}
