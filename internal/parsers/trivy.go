package parsers

import (
	"encoding/json"
	"fmt"
)

// trivyReport mirrors the top-level JSON object produced by
// `trivy fs --format json`. Only the fields PipelineGuard needs are declared;
// everything else (SchemaVersion, ArtifactName, ...) is ignored.
//
// Results may be null when nothing was scanned, in which case the slice stays
// nil and no findings are produced.
type trivyReport struct {
	Results []trivyResult `json:"Results"`
}

// trivyResult is one scanned target inside a trivy report. A target with no
// issues may omit "Vulnerabilities" entirely, so Vulnerabilities can be nil.
type trivyResult struct {
	Target          string               `json:"Target"`
	Vulnerabilities []trivyVulnerability `json:"Vulnerabilities"`
}

// trivyVulnerability is a single dependency vulnerability reported by trivy.
type trivyVulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	Title            string `json:"Title"`
	Severity         string `json:"Severity"`
}

// ParseTrivy takes the raw JSON emitted by `trivy fs --format json` (dependency
// scan) and returns the normalized findings.
//
// Edge cases handled without error:
//   - "Results": null            -> empty slice, no findings
//   - a Result without "Vulnerabilities" -> skipped, no findings
//
// Malformed JSON produces an error rather than a panic.
//
// Field mapping (trivy -> Finding):
//
//	Tool     = "trivy" (constant)
//	Severity = normalizeTrivySeverity(Severity)
//	File     = Target (of the enclosing Result)
//	Line     = 0 (trivy reports no line for dependency vulnerabilities)
//	Rule     = VulnerabilityID
//	Message  = "<Title> (<PkgName>@<InstalledVersion>)"
func ParseTrivy(raw []byte) ([]Finding, error) {
	var report trivyReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("parsers: parsing trivy report: %w", err)
	}

	findings := make([]Finding, 0)
	for _, result := range report.Results {
		for _, vuln := range result.Vulnerabilities {
			findings = append(findings, Finding{
				Tool:     "trivy",
				Severity: normalizeTrivySeverity(vuln.Severity),
				File:     result.Target,
				Line:     0,
				Rule:     vuln.VulnerabilityID,
				Message: fmt.Sprintf("%s (%s@%s)",
					vuln.Title, vuln.PkgName, vuln.InstalledVersion),
			})
		}
	}

	return findings, nil
}

// normalizeTrivySeverity maps trivy's native severity to the unified enum.
// CRITICAL/HIGH/MEDIUM/LOW map 1:1; UNKNOWN and any unrecognized value
// (for future robustness) normalize to LOW.
func normalizeTrivySeverity(sev string) string {
	switch sev {
	case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		return sev
	default:
		return "LOW"
	}
}
