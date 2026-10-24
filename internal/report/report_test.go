package report

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestEscapeTableCell(t *testing.T) {
	assert.Equal(t, "no pipes here", escapeTableCell("no pipes here"))
	assert.Equal(t, `path A\|path B`, escapeTableCell("path A|path B"))
	assert.Equal(t, `\|a\|\|b\|`, escapeTableCell("|a||b|"))
	assert.Equal(t, "", escapeTableCell(""))
}

// countUnescapedPipes counts the "|" characters in line that are real column
// separators, i.e. not preceded by a backslash.
func countUnescapedPipes(line string) int {
	n := 0
	for i, r := range line {
		if r == '|' && (i == 0 || line[i-1] != '\\') {
			n++
		}
	}
	return n
}

func TestGenerateMarkdown_PipeInFieldsIsEscaped(t *testing.T) {
	findings := []parsers.Finding{
		{
			Tool:     "trivy",
			Severity: "HIGH",
			File:     "dir|with|pipes/go.sum",
			Line:     0,
			Rule:     "CVE|2024",
			Message:  "path A|path B",
		},
	}
	out := GenerateMarkdown(findings, 5, map[string]int{"HIGH": 1})

	assert.Contains(t, out, `path A\|path B`)
	assert.Contains(t, out, `dir\|with\|pipes/go.sum:0`)
	assert.Contains(t, out, `CVE\|2024`)

	var header, row string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "| Severidad | Herramienta"):
			header = line
		case strings.HasPrefix(line, "| HIGH | trivy"):
			row = line
		}
	}
	require.NotEmpty(t, header, "detail table header not found")
	require.NotEmpty(t, row, "detail table row not found")

	// 5 columns -> 6 separators, and the finding row must match the header.
	assert.Equal(t, 6, countUnescapedPipes(header))
	assert.Equal(t, countUnescapedPipes(header), countUnescapedPipes(row),
		"a '|' inside a field must not add columns to the row: %q", row)
}

// detailRows returns the detail-table rows of a generated report (5 columns,
// excluding the header and the separator line), in output order.
func detailRows(out string) []string {
	var rows []string
	for _, line := range strings.Split(out, "\n") {
		if countUnescapedPipes(line) == 6 &&
			!strings.HasPrefix(line, "| Severidad |") && !strings.HasPrefix(line, "|---") {
			rows = append(rows, line)
		}
	}
	return rows
}

// manyFindings returns n CRITICAL findings with distinct, sortable file names.
func manyFindings(n int, message string) []parsers.Finding {
	findings := make([]parsers.Finding, n)
	for i := range findings {
		findings[i] = parsers.Finding{
			Tool: "gitleaks", Severity: "CRITICAL",
			File: fmt.Sprintf("f%04d.py", i), Line: 1, Rule: "r", Message: message,
		}
	}
	return findings
}

func TestGenerateMarkdown_NoFindingsNoTruncationNotice(t *testing.T) {
	out := GenerateMarkdown(nil, 0, map[string]int{})

	assert.Empty(t, detailRows(out))
	assert.NotContains(t, out, "se omitieron")
}

func TestGenerateMarkdown_ExactlyMaxDetailRowsNotTruncated(t *testing.T) {
	out := GenerateMarkdown(manyFindings(maxDetailRows, "m"), 0, map[string]int{"CRITICAL": maxDetailRows})

	assert.Len(t, detailRows(out), maxDetailRows)
	assert.NotContains(t, out, "se omitieron")
}

func TestGenerateMarkdown_OverMaxDetailRowsOmitsLeastSevere(t *testing.T) {
	// The single LOW finding comes first in the input, but after sorting it is
	// the least severe row, so it is the one left out.
	findings := append([]parsers.Finding{
		{Tool: "trivy", Severity: "LOW", File: "a.mod", Line: 0, Rule: "CVE-LOW", Message: "low"},
	}, manyFindings(maxDetailRows, "m")...)
	out := GenerateMarkdown(findings, 0, map[string]int{"CRITICAL": maxDetailRows, "LOW": 1})

	rows := detailRows(out)
	assert.Len(t, rows, maxDetailRows)
	assert.NotContains(t, out, "CVE-LOW", "the least severe finding must be the one omitted")
	for _, r := range rows {
		assert.True(t, strings.HasPrefix(r, "| CRITICAL |"), "unexpected row kept: %q", r)
	}
	// Sort within a severity (by file) is preserved after truncating.
	assert.True(t, strings.HasPrefix(rows[0], "| CRITICAL | gitleaks | f0000.py:1 |"))
	assert.True(t, strings.HasPrefix(rows[len(rows)-1], fmt.Sprintf("| CRITICAL | gitleaks | f%04d.py:1 |", maxDetailRows-1)))

	assert.Contains(t, out, fmt.Sprintf("Se muestran %d de %d hallazgos", maxDetailRows, maxDetailRows+1))
	assert.Contains(t, out, "se omitieron 1 ")
	assert.Contains(t, out, "**Security**")
	assert.Contains(t, out, "SARIF")
}

func TestGenerateMarkdown_HugeMessagesTriggerSafetyNet(t *testing.T) {
	// 10 rows of ~10,000 characters each: under maxDetailRows, but far over
	// maxMarkdownChars, so the safety net has to drop rows.
	findings := append(manyFindings(9, strings.Repeat("x", 10000)),
		parsers.Finding{Tool: "trivy", Severity: "HIGH", File: "go.sum", Line: 0, Rule: "CVE-HIGH", Message: strings.Repeat("y", 10000)})
	out := GenerateMarkdown(findings, 0, map[string]int{"CRITICAL": 9, "HIGH": 1})

	assert.LessOrEqual(t, utf8.RuneCountInString(out), maxMarkdownChars)
	rows := detailRows(out)
	require.NotEmpty(t, rows)
	assert.Less(t, len(rows), len(findings))
	assert.NotContains(t, out, "CVE-HIGH", "the HIGH row sorts last, so it is dropped first")
	assert.Contains(t, out, fmt.Sprintf("Se muestran %d de %d hallazgos", len(rows), len(findings)))
	assert.Contains(t, out, fmt.Sprintf("se omitieron %d ", len(findings)-len(rows)))
}

func TestGenerateMarkdown_LimitCountsCharactersNotBytes(t *testing.T) {
	// 5 × 10,000 "é" = ~50,000 characters but ~100,000 bytes: it fits in
	// characters, so nothing may be dropped.
	out := GenerateMarkdown(manyFindings(5, strings.Repeat("é", 10000)), 0, map[string]int{"CRITICAL": 5})

	assert.Greater(t, len(out), maxMarkdownChars, "test premise: the report is over the limit in bytes")
	assert.LessOrEqual(t, utf8.RuneCountInString(out), maxMarkdownChars)
	assert.Len(t, detailRows(out), 5)
	assert.NotContains(t, out, "se omitieron")
}
