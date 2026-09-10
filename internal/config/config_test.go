package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pipelineguard/internal/scoring"
)

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".pipelineguard.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yml")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, DefaultConfig(), cfg)
}

func TestLoad_FullYAMLOverridesEveryField(t *testing.T) {
	path := writeConfig(t, `
scanners:
  gitleaks: false
  trivy: false
  semgrep: true
enforce: true
fail_threshold: HIGH
ignore_paths:
  - "**/vendor/**"
  - "**/testdata/**"
severity_weights:
  CRITICAL: 100
  HIGH: 50
  MEDIUM: 20
  LOW: 10
  INFO: 5
`)

	cfg, err := Load(path)
	require.NoError(t, err)

	want := Config{
		Scanners: ScannersConfig{
			Gitleaks: false,
			Trivy:    false,
			Semgrep:  true,
		},
		Enforce:       true,
		FailThreshold: "HIGH",
		IgnorePaths:   []string{"**/vendor/**", "**/testdata/**"},
		SeverityWeights: map[string]int{
			"CRITICAL": 100,
			"HIGH":     50,
			"MEDIUM":   20,
			"LOW":      10,
			"INFO":     5,
		},
	}
	assert.Equal(t, want, cfg)
}

func TestLoad_PartialYAMLKeepsDefaults(t *testing.T) {
	path := writeConfig(t, "enforce: true\n")

	cfg, err := Load(path)
	require.NoError(t, err)

	want := DefaultConfig()
	want.Enforce = true
	assert.Equal(t, want, cfg)
}

// TestLoad_NestedScannerMergePreservesDefaults empirically checks that setting a
// single key under `scanners` does not wipe the sibling keys' defaults.
func TestLoad_NestedScannerMergePreservesDefaults(t *testing.T) {
	path := writeConfig(t, `
scanners:
  gitleaks: false
`)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.False(t, cfg.Scanners.Gitleaks, "explicitly set to false")
	assert.True(t, cfg.Scanners.Trivy, "unset key must keep its default (true)")
	assert.False(t, cfg.Scanners.Semgrep, "unset key must keep its default (false)")

	// Everything outside the scanners block is untouched.
	want := DefaultConfig()
	want.Scanners.Gitleaks = false
	assert.Equal(t, want, cfg)
}

// TestLoad_SeverityWeightsMergePreservesDefaults empirically checks that setting
// a single severity weight does not drop the other default weights.
func TestLoad_SeverityWeightsMergePreservesDefaults(t *testing.T) {
	path := writeConfig(t, `
severity_weights:
  CRITICAL: 20
`)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 20, cfg.SeverityWeights["CRITICAL"], "explicitly overridden")
	assert.Equal(t, scoring.DefaultSeverityWeights["HIGH"], cfg.SeverityWeights["HIGH"])
	assert.Equal(t, scoring.DefaultSeverityWeights["MEDIUM"], cfg.SeverityWeights["MEDIUM"])
	assert.Equal(t, scoring.DefaultSeverityWeights["LOW"], cfg.SeverityWeights["LOW"])
	assert.Equal(t, scoring.DefaultSeverityWeights["INFO"], cfg.SeverityWeights["INFO"])
	assert.Len(t, cfg.SeverityWeights, len(scoring.DefaultSeverityWeights))
}

// TestLoad_DefaultSeverityWeightsNotMutated guards against Load corrupting the
// shared scoring.DefaultSeverityWeights map through aliasing.
func TestLoad_DefaultSeverityWeightsNotMutated(t *testing.T) {
	path := writeConfig(t, "severity_weights:\n  CRITICAL: 999\n")

	_, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 10, scoring.DefaultSeverityWeights["CRITICAL"],
		"scoring.DefaultSeverityWeights must be unchanged")
}

func TestLoad_InvalidFailThresholdReturnsError(t *testing.T) {
	path := writeConfig(t, `fail_threshold: "BANANA"`)

	_, err := Load(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "BANANA")
}

func TestLoad_LowercaseFailThresholdReturnsError(t *testing.T) {
	// Validation is case-sensitive.
	path := writeConfig(t, "fail_threshold: critical\n")

	_, err := Load(path)
	require.Error(t, err)
}

func TestLoad_MalformedYAMLReturnsError(t *testing.T) {
	path := writeConfig(t, "scanners: [this is not: valid: mapping\nenforce: : :\n")

	assert.NotPanics(t, func() {
		_, err := Load(path)
		assert.Error(t, err)
	})
}
