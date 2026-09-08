package scoring

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"pipelineguard/internal/parsers"
)

func f(severity string) parsers.Finding {
	return parsers.Finding{Tool: "test", Severity: severity}
}

func TestComputeScore_EmptySlice(t *testing.T) {
	assert.Equal(t, 0, ComputeScore(nil, DefaultSeverityWeights))
	assert.Equal(t, 0, ComputeScore([]parsers.Finding{}, DefaultSeverityWeights))
}

func TestCountBySeverity_EmptySlice(t *testing.T) {
	counts := CountBySeverity([]parsers.Finding{})
	assert.NotNil(t, counts)
	assert.Empty(t, counts)
}

func TestComputeScore_OneOfEachKnownSeverity(t *testing.T) {
	findings := []parsers.Finding{
		f("CRITICAL"), // 10
		f("HIGH"),     // 5
		f("MEDIUM"),   // 2
		f("LOW"),      // 1
		f("INFO"),     // 0
	}
	assert.Equal(t, 18, ComputeScore(findings, DefaultSeverityWeights))

	assert.Equal(t, map[string]int{
		"CRITICAL": 1,
		"HIGH":     1,
		"MEDIUM":   1,
		"LOW":      1,
		"INFO":     1,
	}, CountBySeverity(findings))
}

func TestComputeScore_MultipleSameSeverity(t *testing.T) {
	findings := []parsers.Finding{
		f("CRITICAL"), f("CRITICAL"), f("CRITICAL"),
		f("MEDIUM"), f("MEDIUM"),
	}
	// 3*10 + 2*2 = 34
	assert.Equal(t, 34, ComputeScore(findings, DefaultSeverityWeights))
	assert.Equal(t, map[string]int{"CRITICAL": 3, "MEDIUM": 2}, CountBySeverity(findings))
}

func TestComputeScore_UnknownSeverityContributesZero(t *testing.T) {
	findings := []parsers.Finding{
		f("CRITICAL"),       // 10
		f("SUPER-CRITICAL"), // not a key in weights -> 0
		f("LOW"),            // 1
	}
	assert.NotPanics(t, func() {
		assert.Equal(t, 11, ComputeScore(findings, DefaultSeverityWeights))
	})

	// It is still counted by CountBySeverity.
	assert.Equal(t, map[string]int{
		"CRITICAL":       1,
		"SUPER-CRITICAL": 1,
		"LOW":            1,
	}, CountBySeverity(findings))
}

func TestComputeScore_CustomWeightsAreRespected(t *testing.T) {
	custom := map[string]int{
		"CRITICAL": 100,
		"MEDIUM":   7,
		// LOW deliberately omitted -> contributes 0
	}
	findings := []parsers.Finding{
		f("CRITICAL"), // 100
		f("MEDIUM"),   // 7
		f("LOW"),      // 0 with custom weights
	}
	assert.Equal(t, 107, ComputeScore(findings, custom))

	// Sanity check: the default weights would have produced a different score.
	assert.Equal(t, 13, ComputeScore(findings, DefaultSeverityWeights))
}
