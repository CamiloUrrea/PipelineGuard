package parsers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTrivySeverity(t *testing.T) {
	cases := map[string]string{
		"CRITICAL":        "CRITICAL",
		"HIGH":            "HIGH",
		"MEDIUM":          "MEDIUM",
		"LOW":             "LOW",
		"UNKNOWN":         "LOW",
		"":                "LOW",
		"totally-made-up": "LOW",
	}
	for in, want := range cases {
		assert.Equalf(t, want, normalizeTrivySeverity(in), "normalizeTrivySeverity(%q)", in)
	}
}

func TestParseTrivy_MapsFieldsFromFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "trivy_sample.json"))
	require.NoError(t, err)

	findings, err := ParseTrivy(raw)
	require.NoError(t, err)

	want := []Finding{
		{
			Tool:     "trivy",
			Severity: "CRITICAL",
			File:     "go.sum",
			Line:     0,
			Rule:     "CVE-2024-12345",
			Message:  "example: buffer overflow in parser (github.com/example/vulnerable-pkg@1.2.3)",
		},
		{
			Tool:     "trivy",
			Severity: "MEDIUM",
			File:     "go.sum",
			Line:     0,
			Rule:     "CVE-2023-99999",
			Message:  "example: information disclosure via verbose errors (github.com/example/leaky-lib@0.5.0)",
		},
		{
			Tool:     "trivy",
			Severity: "LOW", // UNKNOWN normalized to LOW
			File:     "go.sum",
			Line:     0,
			Rule:     "GHSA-xxxx-yyyy-zzzz",
			Message:  "example: unspecified issue pending analysis (github.com/example/mystery-mod@2.0.1)",
		},
	}

	assert.Equal(t, want, findings)
}

func TestParseTrivy_CleanTargetProducesNoFindings(t *testing.T) {
	// The fixture's second Result ("package-lock.json") has no "Vulnerabilities"
	// field. It must not error and must not contribute findings.
	raw, err := os.ReadFile(filepath.Join("testdata", "trivy_sample.json"))
	require.NoError(t, err)

	findings, err := ParseTrivy(raw)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotEqual(t, "package-lock.json", f.File)
	}
	assert.Len(t, findings, 3)
}

func TestParseTrivy_AggregatesMultipleResults(t *testing.T) {
	raw := []byte(`{
	  "Results": [
	    {"Target": "go.sum", "Vulnerabilities": [
	      {"VulnerabilityID": "CVE-1", "PkgName": "a", "InstalledVersion": "1.0.0", "Title": "t1", "Severity": "HIGH"}
	    ]},
	    {"Target": "requirements.txt", "Vulnerabilities": [
	      {"VulnerabilityID": "CVE-2", "PkgName": "b", "InstalledVersion": "2.0.0", "Title": "t2", "Severity": "LOW"},
	      {"VulnerabilityID": "CVE-3", "PkgName": "c", "InstalledVersion": "3.0.0", "Title": "t3", "Severity": "CRITICAL"}
	    ]}
	  ]
	}`)

	findings, err := ParseTrivy(raw)
	require.NoError(t, err)
	require.Len(t, findings, 3)

	assert.Equal(t, "go.sum", findings[0].File)
	assert.Equal(t, "CVE-1", findings[0].Rule)
	assert.Equal(t, "requirements.txt", findings[1].File)
	assert.Equal(t, "requirements.txt", findings[2].File)
	assert.Equal(t, "CRITICAL", findings[2].Severity)
}

func TestParseTrivy_ResultsNull(t *testing.T) {
	findings, err := ParseTrivy([]byte(`{"SchemaVersion": 2, "Results": null}`))
	require.NoError(t, err)
	assert.Empty(t, findings)
	assert.NotNil(t, findings, "expected a non-nil empty slice")
}

func TestParseTrivy_ResultsMissing(t *testing.T) {
	findings, err := ParseTrivy([]byte(`{"SchemaVersion": 2}`))
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestParseTrivy_MalformedJSON(t *testing.T) {
	findings, err := ParseTrivy([]byte(`{"Results": [ this is not json`))
	assert.Error(t, err)
	assert.Nil(t, findings)
}
