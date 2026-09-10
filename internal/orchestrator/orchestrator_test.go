package orchestrator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pipelineguard/internal/config"
	"pipelineguard/internal/parsers"
	"pipelineguard/internal/policy"
)

const gitleaksJSON = `[
  {"Description": "Generic API Key", "StartLine": 12, "File": "config/settings.py", "RuleID": "generic-api-key"}
]`

const trivyJSON = `{
  "Results": [
    {
      "Target": "go.sum",
      "Vulnerabilities": [
        {"VulnerabilityID": "CVE-2024-1", "PkgName": "p", "InstalledVersion": "1.0.0", "Title": "boom", "Severity": "HIGH"}
      ]
    }
  ]
}`

// bytesFunc returns a ScannerFunc yielding fixed bytes and no error.
func bytesFunc(b string) ScannerFunc {
	return func() ([]byte, error) { return []byte(b), nil }
}

// errFunc returns a ScannerFunc that always fails.
func errFunc(err error) ScannerFunc {
	return func() ([]byte, error) { return nil, err }
}

// mustNotCall fails the test if the scanner is ever invoked.
func mustNotCall(t *testing.T) ScannerFunc {
	t.Helper()
	return func() ([]byte, error) {
		t.Fatal("scanner was called but should have been disabled")
		return nil, nil
	}
}

func cfgWith(gitleaks, trivy bool) config.Config {
	c := config.DefaultConfig()
	c.Scanners.Gitleaks = gitleaks
	c.Scanners.Trivy = trivy
	c.Scanners.Semgrep = false
	return c
}

func TestRun_BothScannersCombineFindings(t *testing.T) {
	res, err := Run(cfgWith(true, true), bytesFunc(gitleaksJSON), bytesFunc(trivyJSON))
	require.NoError(t, err)

	require.Len(t, res.Findings, 2)

	tools := map[string]int{}
	for _, f := range res.Findings {
		tools[f.Tool]++
	}
	assert.Equal(t, 1, tools["gitleaks"])
	assert.Equal(t, 1, tools["trivy"])

	assert.Equal(t, map[string]int{"CRITICAL": 1, "HIGH": 1}, res.Counts)
	assert.Contains(t, res.Markdown, "Risk Score:")
	assert.NotEmpty(t, res.SARIF)
	assert.NotEmpty(t, res.Reason)
}

func TestRun_GitleaksDisabledNeverCallsScanner(t *testing.T) {
	called := false
	spyGitleaks := func() ([]byte, error) {
		called = true
		return []byte(gitleaksJSON), nil
	}

	res, err := Run(cfgWith(false, true), spyGitleaks, bytesFunc(trivyJSON))
	require.NoError(t, err)

	assert.False(t, called, "runGitleaks must not be invoked when disabled")
	for _, f := range res.Findings {
		assert.NotEqual(t, "gitleaks", f.Tool, "no gitleaks findings expected")
	}
	require.Len(t, res.Findings, 1)
	assert.Equal(t, "trivy", res.Findings[0].Tool)
}

func TestRun_TrivyDisabledNeverCallsScanner(t *testing.T) {
	called := false
	spyTrivy := func() ([]byte, error) {
		called = true
		return []byte(trivyJSON), nil
	}

	res, err := Run(cfgWith(true, false), bytesFunc(gitleaksJSON), spyTrivy)
	require.NoError(t, err)

	assert.False(t, called, "runTrivy must not be invoked when disabled")
	for _, f := range res.Findings {
		assert.NotEqual(t, "trivy", f.Tool, "no trivy findings expected")
	}
	require.Len(t, res.Findings, 1)
	assert.Equal(t, "gitleaks", res.Findings[0].Tool)
}

func TestRun_GitleaksScanErrorIsSurfaced(t *testing.T) {
	sentinel := errors.New("gitleaks exited 2")

	res, err := Run(cfgWith(true, true), errFunc(sentinel), mustNotCall(t))

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "gitleaks")
	assert.Equal(t, Result{}, res, "no silent empty result on scan failure")
}

func TestRun_GitleaksMalformedJSONIsError(t *testing.T) {
	assert.NotPanics(t, func() {
		res, err := Run(cfgWith(true, false), bytesFunc("{not json"), mustNotCall(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gitleaks")
		assert.Equal(t, Result{}, res)
	})
}

func TestRun_TrivyScanErrorIsSurfaced(t *testing.T) {
	sentinel := errors.New("trivy exited 1")

	// gitleaks disabled so trivy is the only scanner run.
	_, err := Run(cfgWith(false, true), mustNotCall(t), errFunc(sentinel))

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "trivy")
}

func TestRun_BothScannersDisabled(t *testing.T) {
	res, err := Run(cfgWith(false, false), mustNotCall(t), mustNotCall(t))
	require.NoError(t, err)

	assert.Empty(t, res.Findings)
	assert.Equal(t, 0, res.Score)
	assert.Empty(t, res.Counts)
	assert.NotEmpty(t, res.SARIF, "SARIF is still a valid empty document")
	assert.NotEmpty(t, res.Reason)
}

func TestRun_SemgrepFlagIgnored(t *testing.T) {
	c := cfgWith(false, false)
	c.Scanners.Semgrep = true // must be ignored, not an error

	res, err := Run(c, mustNotCall(t), mustNotCall(t))
	require.NoError(t, err)
	assert.Empty(t, res.Findings)
}

func TestRun_EnforceShouldFailMatchesPolicy(t *testing.T) {
	c := cfgWith(true, true)
	c.Enforce = true
	c.FailThreshold = "HIGH"

	res, err := Run(c, bytesFunc(gitleaksJSON), bytesFunc(trivyJSON))
	require.NoError(t, err)

	wantFail, wantReason := policy.ShouldFail(res.Findings, c)
	assert.True(t, res.ShouldFail)
	assert.Equal(t, wantFail, res.ShouldFail)
	assert.Equal(t, wantReason, res.Reason)
}

func TestRun_EnforceBelowThresholdDoesNotFail(t *testing.T) {
	c := cfgWith(false, true)
	c.Enforce = true
	c.FailThreshold = "CRITICAL"

	// trivy fixture only has a HIGH finding, below the CRITICAL threshold.
	res, err := Run(c, mustNotCall(t), bytesFunc(trivyJSON))
	require.NoError(t, err)

	assert.False(t, res.ShouldFail)
	assert.NotEmpty(t, res.Reason)

	// Sanity check the finding really is a parsers.Finding of severity HIGH.
	require.Len(t, res.Findings, 1)
	assert.Equal(t, "HIGH", res.Findings[0].Severity)
	assert.IsType(t, parsers.Finding{}, res.Findings[0])
}
