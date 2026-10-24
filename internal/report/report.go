// Package report renders a slice of normalized findings (plus the pre-computed
// risk score and per-severity counts) into a Markdown report meant to be posted
// as a pull-request comment.
package report

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"pipelineguard/internal/parsers"
)

// maxDetailRows caps the detail table. Rows are sorted by severity first, so
// whatever is left out is the least severe. The SARIF is never truncated.
const maxDetailRows = 100

// maxMarkdownChars is the safety net below GitHub's 65,536-character limit for
// a comment body: if the capped table is still too long (very long messages),
// rows are dropped until it fits. It counts characters (runes), not bytes,
// because that is how GitHub measures the limit.
const maxMarkdownChars = 60000

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

// escapeTableCell makes s safe to place inside a Markdown table cell: a literal
// "|" would otherwise be read as a column separator and break the row, so it is
// escaped as "\|".
func escapeTableCell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// GenerateMarkdown builds the PR-comment Markdown report.
//
//   - Always: a title line with the total risk score.
//   - If countsBySeverity is non-empty: a summary table of counts, in
//     severityDisplayOrder, listing only the severities present.
//   - If findings is empty: a short "no findings" line (no empty table).
//   - Otherwise: a detail table sorted by severity (severityDisplayOrder) and,
//     within a severity, by file name. Every text cell goes through
//     escapeTableCell so a "|" inside a finding cannot break the table.
//   - The detail table lists at most maxDetailRows rows and the whole report
//     stays within maxMarkdownChars; when rows are omitted, a notice says how
//     many and points to the Security tab / SARIF for the full list.
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

	rows := make([]string, len(sorted))
	for i, f := range sorted {
		rows[i] = fmt.Sprintf("| %s | %s | %s:%d | %s | %s |\n",
			escapeTableCell(f.Severity), escapeTableCell(f.Tool), escapeTableCell(f.File),
			f.Line, escapeTableCell(f.Rule), escapeTableCell(f.Message))
	}

	const tableHeader = "\n| Severidad | Herramienta | Archivo:Línea | Regla | Mensaje |\n|---|---|---|---|---|\n"

	// Keep at most maxDetailRows, then drop rows from the end (least severe)
	// until the whole comment fits in maxMarkdownChars.
	shown := min(len(rows), maxDetailRows)
	length := utf8.RuneCountInString(b.String()) + utf8.RuneCountInString(tableHeader)
	for _, r := range rows[:shown] {
		length += utf8.RuneCountInString(r)
	}
	for shown > 0 && length+utf8.RuneCountInString(truncationNotice(shown, len(rows))) > maxMarkdownChars {
		shown--
		length -= utf8.RuneCountInString(rows[shown])
	}

	if shown > 0 {
		b.WriteString(tableHeader)
		for _, r := range rows[:shown] {
			b.WriteString(r)
		}
	}
	b.WriteString(truncationNotice(shown, len(rows)))

	return b.String()
}

// truncationNotice is the line appended below the detail table when only shown
// of total findings are listed; it is empty when nothing was omitted.
func truncationNotice(shown, total int) string {
	if shown >= total {
		return ""
	}
	return fmt.Sprintf("\n> ⚠️ Se muestran %d de %d hallazgos (los de mayor severidad); se omitieron %d "+
		"para no exceder el límite de tamaño de los comentarios de GitHub. "+
		"El detalle completo está en la pestaña **Security** del repositorio y en el archivo SARIF.\n",
		shown, total, total-shown)
}
