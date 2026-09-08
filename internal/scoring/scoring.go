// Package scoring turns a slice of normalized findings into aggregate numbers:
// a weighted risk score and a per-severity count. It receives already-parsed
// []parsers.Finding, so there is no raw input to validate here.
package scoring

import "pipelineguard/internal/parsers"

// DefaultSeverityWeights is the default weight of each severity in the risk
// score (see ARCHITECTURE.md, "Modelo de severidad").
var DefaultSeverityWeights = map[string]int{
	"CRITICAL": 10,
	"HIGH":     5,
	"MEDIUM":   2,
	"LOW":      1,
	"INFO":     0,
}

// ComputeScore sums the weight of every finding's severity. A severity that is
// not a key in weights contributes 0 (it is still visited, just not scored).
func ComputeScore(findings []parsers.Finding, weights map[string]int) int {
	score := 0
	for _, f := range findings {
		score += weights[f.Severity]
	}
	return score
}

// CountBySeverity returns how many findings there are per severity present.
// Severities with zero findings are absent from the returned map. The map is
// non-nil even when findings is empty.
func CountBySeverity(findings []parsers.Finding) map[string]int {
	counts := make(map[string]int)
	for _, f := range findings {
		counts[f.Severity]++
	}
	return counts
}
