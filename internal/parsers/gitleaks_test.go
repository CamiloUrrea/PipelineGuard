package parsers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitleaks_MapsEveryField(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "gitleaks_sample.json"))
	require.NoError(t, err)

	findings, err := ParseGitleaks(raw)
	require.NoError(t, err)
	require.Len(t, findings, 3)

	want := []Finding{
		{
			Tool:     "gitleaks",
			Severity: "CRITICAL",
			File:     "config/settings.py",
			Line:     12,
			Rule:     "generic-api-key",
			Message:  "Generic API Key",
		},
		{
			Tool:     "gitleaks",
			Severity: "CRITICAL",
			File:     "deploy/terraform/main.tf",
			Line:     47,
			Rule:     "aws-access-token",
			Message:  "AWS Access Key",
		},
		{
			Tool:     "gitleaks",
			Severity: "CRITICAL",
			File:     ".env.example",
			Line:     3,
			Rule:     "github-pat",
			Message:  "GitHub Personal Access Token",
		},
	}

	assert.Equal(t, want, findings)
}

func TestParseGitleaks_EmptyArray(t *testing.T) {
	findings, err := ParseGitleaks([]byte("[]"))
	require.NoError(t, err)
	assert.Empty(t, findings)
	assert.NotNil(t, findings, "expected a non-nil empty slice")
}

func TestParseGitleaks_MalformedJSON(t *testing.T) {
	findings, err := ParseGitleaks([]byte("{not valid json"))
	assert.Error(t, err)
	assert.Nil(t, findings)
}

func TestParseGitleaks_AllFindingsAreCritical(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "gitleaks_sample.json"))
	require.NoError(t, err)

	findings, err := ParseGitleaks(raw)
	require.NoError(t, err)

	for _, f := range findings {
		assert.Equal(t, "CRITICAL", f.Severity)
		assert.Equal(t, "gitleaks", f.Tool)
	}
}
