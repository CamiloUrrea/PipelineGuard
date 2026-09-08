// Package report renders a slice of normalized findings (plus the pre-computed
// risk score and per-severity counts) into a Markdown report meant to be posted
// as a pull-request comment.
package report

import (
	"fmt"
	"sort"
	"strings"

	"pipelineguard/internal/parsers"
)

// severityDisplayOrder is the fixed order severities are presented in, both in
// the summary table and the detail table. This is a presentation concern only
// and is deliberately kept separate from internal/scoring's weights.
var severityDisplayOrder = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"}

// severityRank returns the position of a severity in severityDisplayOrder.
// Unrecognized severities sort after every known one (in their original order).
func severityRank(severity string) int {
	for i, s := range severityDisplayOrder {
		if s == severity {
			return i
		}
	}
	return len(severityDisplayOrder)
}

// GenerateMarkdown builds the PR-comment Markdown report.
//
//   - Always: a title line with the total risk score.
//   - If countsBySeverity is non-empty: a summary table of counts, in
//     severityDisplayOrder, listing only the severities present.
//   - If findings is empty: a short "no findings" line (no empty table).
//   - Otherwise: a detail table sorted by severity (severityDisplayOrder) and,
//     within a severity, by file name.
func GenerateMarkdown(findings []parsers.Finding, score int, countsBySeverity map[string]int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## 🛡️ PipelineGuard — Risk Score: %d\n", score)

	if len(countsBySeverity) > 0 {
		b.WriteString("\n| Severidad | Hallazgos |\n")
		b.WriteString("|---|---|\n")
		for _, sev := range severityDisplayOrder {
			if n, ok := countsBySeverity[sev]; ok {
				fmt.Fprintf(&b, "| %s | %d |\n", sev, n)
			}
		}
	}

	if len(findings) == 0 {
		b.WriteString("\n✅ No se encontraron hallazgos.\n")
		return b.String()
	}

	sorted := make([]parsers.Finding, len(findings))
	copy(sorted, findings)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := severityRank(sorted[i].Severity), severityRank(sorted[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return sorted[i].File < sorted[j].File
	})

	b.WriteString("\n| Severidad | Herramienta | Archivo:Línea | Regla | Mensaje |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, f := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s:%d | %s | %s |\n",
			f.Severity, f.Tool, f.File, f.Line, f.Rule, f.Message)
	}

	return b.String()
}
