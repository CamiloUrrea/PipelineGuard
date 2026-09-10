package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"pipelineguard/internal/config"
	"pipelineguard/internal/parsers"
)

func finding(sev string) parsers.Finding {
	return parsers.Finding{Tool: "trivy", Severity: sev, File: "go.sum", Rule: "CVE-X", Message: "m"}
}

func cfg(enforce bool, threshold string) config.Config {
	c := config.DefaultConfig()
	c.Enforce = enforce
	c.FailThreshold = threshold
	return c
}

func TestShouldFail_InformationalModeNeverFails(t *testing.T) {
	findings := []parsers.Finding{finding("CRITICAL"), finding("CRITICAL")}

	fail, reason := ShouldFail(findings, cfg(false, "CRITICAL"))

	assert.False(t, fail)
	assert.NotEmpty(t, reason)
}

func TestShouldFail_EnforceCriticalWithCritical(t *testing.T) {
	findings := []parsers.Finding{finding("LOW"), finding("CRITICAL")}

	fail, reason := ShouldFail(findings, cfg(true, "CRITICAL"))

	assert.True(t, fail)
	assert.NotEmpty(t, reason)
}

func TestShouldFail_EnforceCriticalWithoutCritical(t *testing.T) {
	findings := []parsers.Finding{finding("HIGH"), finding("MEDIUM"), finding("LOW")}

	fail, reason := ShouldFail(findings, cfg(true, "CRITICAL"))

	assert.False(t, fail)
	assert.NotEmpty(t, reason)
}

func TestShouldFail_EnforceMediumWithHigherSeverity(t *testing.T) {
	// HIGH (rank 3) is above the MEDIUM (rank 2) threshold.
	findings := []parsers.Finding{finding("HIGH")}

	fail, reason := ShouldFail(findings, cfg(true, "MEDIUM"))

	assert.True(t, fail)
	assert.NotEmpty(t, reason)
}

func TestShouldFail_EnforceMediumExactMatch(t *testing.T) {
	// The threshold itself must count: rank >= threshold.
	findings := []parsers.Finding{finding("MEDIUM")}

	fail, reason := ShouldFail(findings, cfg(true, "MEDIUM"))

	assert.True(t, fail)
	assert.NotEmpty(t, reason)
}

func TestShouldFail_EnforceEmptyFindings(t *testing.T) {
	fail, reason := ShouldFail(nil, cfg(true, "CRITICAL"))

	assert.False(t, fail)
	assert.NotEmpty(t, reason)
}

func TestShouldFail_ReasonAlwaysNonEmpty(t *testing.T) {
	cases := []struct {
		name     string
		findings []parsers.Finding
		cfg      config.Config
	}{
		{"informational", []parsers.Finding{finding("CRITICAL")}, cfg(false, "CRITICAL")},
		{"enforce fail", []parsers.Finding{finding("CRITICAL")}, cfg(true, "HIGH")},
		{"enforce pass", []parsers.Finding{finding("LOW")}, cfg(true, "HIGH")},
		{"enforce empty", nil, cfg(true, "MEDIUM")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, reason := ShouldFail(tc.findings, tc.cfg)
			assert.NotEmpty(t, reason)
		})
	}
}
