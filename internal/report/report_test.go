package report

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"pipelineguard/internal/parsers"
)

func TestGenerateMarkdown_NoFindings(t *testing.T) {
	out := GenerateMarkdown(nil, 0, map[string]int{})

	assert.Contains(t, out, "Risk Score: 0")
	assert.Contains(t, out, "No se encontraron hallazgos")
	// No detail table header when there are no findings.
	assert.NotContains(t, out, "Archivo:Línea")
	// No summary table either (empty counts map).
	assert.NotContains(t, out, "| Severidad | Hallazgos |")
}

func TestGenerateMarkdown_SingleFinding(t *testing.T) {
	findings := []parsers.Finding{
		{
			Tool:     "gitleaks",
			Severity: "CRITICAL",
			File:     "config/settings.py",
			Line:     12,
			Rule:     "generic-api-key",
			Message:  "Generic API Key",
		},
	}
	out := GenerateMarkdown(findings, 10, map[string]int{"CRITICAL": 1})

	assert.Contains(t, out, "Risk Score: 10")
	// Summary table.
	assert.Contains(t, out, "| Severidad | Hallazgos |")
	assert.Contains(t, out, "| CRITICAL | 1 |")
	// Detail table row with every field.
	assert.Contains(t, out, "| Severidad | Herramienta | Archivo:Línea | Regla | Mensaje |")
	assert.Contains(t, out, "| CRITICAL | gitleaks | config/settings.py:12 | generic-api-key | Generic API Key |")
}

func TestGenerateMarkdown_MixedSeveritiesOrdering(t *testing.T) {
	findings := []parsers.Finding{
		{Tool: "trivy", Severity: "LOW", File: "go.sum", Line: 0, Rule: "CVE-LOW", Message: "low sev"},
		{Tool: "trivy", Severity: "CRITICAL", File: "go.sum", Line: 0, Rule: "CVE-CRIT", Message: "crit sev"},
		{Tool: "trivy", Severity: "MEDIUM", File: "go.sum", Line: 0, Rule: "CVE-MED", Message: "med sev"},
	}
	out := GenerateMarkdown(findings, 13, map[string]int{"CRITICAL": 1, "MEDIUM": 1, "LOW": 1})

	crit := strings.Index(out, "CVE-CRIT")
	med := strings.Index(out, "CVE-MED")
	low := strings.Index(out, "CVE-LOW")

	assert.NotEqual(t, -1, crit)
	assert.NotEqual(t, -1, med)
	assert.NotEqual(t, -1, low)
	assert.Less(t, crit, med, "CRITICAL row must come before MEDIUM row")
	assert.Less(t, med, low, "MEDIUM row must come before LOW row")
}

func TestGenerateMarkdown_SameSeveritySortedByFile(t *testing.T) {
	findings := []parsers.Finding{
		{Tool: "trivy", Severity: "HIGH", File: "z.mod", Line: 0, Rule: "CVE-Z", Message: "z"},
		{Tool: "trivy", Severity: "HIGH", File: "a.mod", Line: 0, Rule: "CVE-A", Message: "a"},
	}
	out := GenerateMarkdown(findings, 10, map[string]int{"HIGH": 2})

	assert.Less(t, strings.Index(out, "CVE-A"), strings.Index(out, "CVE-Z"),
		"within the same severity, rows must be sorted by file name")
}

func TestGenerateMarkdown_EmptyCountsNoSummaryTable(t *testing.T) {
	findings := []parsers.Finding{
		{Tool: "gitleaks", Severity: "CRITICAL", File: "a.py", Line: 1, Rule: "r", Message: "m"},
	}
	out := GenerateMarkdown(findings, 10, map[string]int{})

	assert.NotContains(t, out, "| Severidad | Hallazgos |")
	// Detail table is still present.
	assert.Contains(t, out, "| Severidad | Herramienta | Archivo:Línea | Regla | Mensaje |")
}
