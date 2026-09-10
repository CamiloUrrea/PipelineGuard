// Package policy decides whether a PipelineGuard run should fail the build,
// given the normalized findings and the loaded configuration. It performs no
// I/O and returns no error: the inputs are already parsed and validated.
package policy

import (
	"fmt"

	"pipelineguard/internal/config"
	"pipelineguard/internal/parsers"
)

// ShouldFail reports whether the build should fail and a human-readable reason
// that always explains the outcome.
//
//   - enforce=false: never fails, regardless of findings (informational mode).
//   - enforce=true: fails if at least one finding's severity rank is >= the rank
//     of cfg.FailThreshold (see parsers.SeverityRank).
func ShouldFail(findings []parsers.Finding, cfg config.Config) (fail bool, reason string) {
	if !cfg.Enforce {
		return false, "modo informativo: enforce=false, el build no falla independientemente de los hallazgos"
	}

	threshold := parsers.SeverityRank(cfg.FailThreshold)

	atOrAbove := 0
	for _, f := range findings {
		if parsers.SeverityRank(f.Severity) >= threshold {
			atOrAbove++
		}
	}

	if atOrAbove > 0 {
		return true, fmt.Sprintf(
			"enforce=true: %d hallazgo(s) igualan o superan el umbral %s; el build falla",
			atOrAbove, cfg.FailThreshold)
	}

	return false, fmt.Sprintf(
		"enforce=true: ningún hallazgo alcanzó el umbral %s; el build no falla",
		cfg.FailThreshold)
}
