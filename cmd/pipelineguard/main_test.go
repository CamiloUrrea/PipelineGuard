package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"pipelineguard/internal/config"
	"pipelineguard/internal/orchestrator"
)

func TestExitCode(t *testing.T) {
	assert.Equal(t, 0, exitCode(nil, false))
	assert.Equal(t, 1, exitCode(nil, true))
	assert.Equal(t, 2, exitCode(errors.New("boom"), false))
	assert.Equal(t, 2, exitCode(errors.New("boom"), true), "a tool error always wins over shouldFail")
}

// okConfig / noWrite / failNow are small helpers for the runApp tests.
func okConfig(string) (config.Config, error) { return config.DefaultConfig(), nil }

func TestRunApp_ConfigLoadErrorReturnsTwoAndSkipsOrchestrator(t *testing.T) {
	orchestratorCalled := false

	code := runApp(
		"missing.yml", "out.sarif",
		func(string) (config.Config, error) { return config.Config{}, errors.New("invalid fail_threshold") },
		func(config.Config) (orchestrator.Result, error) {
			orchestratorCalled = true
			return orchestrator.Result{}, nil
		},
		func(string, []byte) error { return nil },
		&bytes.Buffer{},
	)

	assert.Equal(t, 2, code)
	assert.False(t, orchestratorCalled, "orchestrator must not run when config loading fails")
}

func TestRunApp_OrchestratorErrorReturnsTwo(t *testing.T) {
	writeCalled := false

	code := runApp(
		"x.yml", "out.sarif",
		okConfig,
		func(config.Config) (orchestrator.Result, error) {
			return orchestrator.Result{}, errors.New("running gitleaks scanner: exit 2")
		},
		func(string, []byte) error {
			writeCalled = true
			return nil
		},
		&bytes.Buffer{},
	)

	assert.Equal(t, 2, code)
	assert.False(t, writeCalled, "SARIF must not be written when the scan failed")
}

func TestRunApp_ShouldFailWritesSarifAndReturnsOne(t *testing.T) {
	sarifBytes := []byte(`{"version":"2.1.0","runs":[]}`)
	var gotPath string
	var gotBytes []byte
	writeCalled := false

	var out bytes.Buffer
	code := runApp(
		"x.yml", "results.sarif",
		okConfig,
		func(config.Config) (orchestrator.Result, error) {
			return orchestrator.Result{
				Markdown:   "## 🛡️ PipelineGuard — Risk Score: 10",
				SARIF:      sarifBytes,
				ShouldFail: true,
				Reason:     "enforce=true: 1 hallazgo(s) igualan o superan el umbral CRITICAL; el build falla",
			}, nil
		},
		func(path string, b []byte) error {
			writeCalled = true
			gotPath = path
			gotBytes = b
			return nil
		},
		&out,
	)

	assert.Equal(t, 1, code)
	assert.True(t, writeCalled)
	assert.Equal(t, "results.sarif", gotPath)
	assert.Equal(t, sarifBytes, gotBytes)
	assert.Contains(t, out.String(), "Risk Score: 10", "markdown still goes to stdout on a failing build")
}

func TestRunApp_SuccessReturnsZeroAndPrintsMarkdown(t *testing.T) {
	var out bytes.Buffer
	code := runApp(
		"x.yml", "results.sarif",
		okConfig,
		func(config.Config) (orchestrator.Result, error) {
			return orchestrator.Result{
				Markdown:   "## 🛡️ PipelineGuard — Risk Score: 0\n\n✅ No se encontraron hallazgos.",
				SARIF:      []byte("{}"),
				ShouldFail: false,
			}, nil
		},
		func(string, []byte) error { return nil },
		&out,
	)

	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "No se encontraron hallazgos")
}

func TestRunApp_WriteFileErrorReturnsTwoEvenOnSuccessfulScan(t *testing.T) {
	var out bytes.Buffer
	code := runApp(
		"x.yml", "results.sarif",
		okConfig,
		func(config.Config) (orchestrator.Result, error) {
			return orchestrator.Result{
				Markdown:   "## 🛡️ PipelineGuard — Risk Score: 3",
				SARIF:      []byte("{}"),
				ShouldFail: false,
			}, nil
		},
		func(string, []byte) error { return errors.New("no space left on device") },
		&out,
	)

	assert.Equal(t, 2, code)
	assert.NotContains(t, out.String(), "Risk Score: 3",
		"markdown must not be printed if the SARIF write failed first")
}

// failingWriter is a minimal io.Writer fake whose Write always errors, used
// to simulate a broken stdout (e.g. a closed pipe).
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("broken pipe")
}

func TestRunApp_StdoutWriteErrorReturnsTwoEvenOnSuccessfulScan(t *testing.T) {
	code := runApp(
		"x.yml", "results.sarif",
		okConfig,
		func(config.Config) (orchestrator.Result, error) {
			return orchestrator.Result{
				Markdown:   "## 🛡️ PipelineGuard — Risk Score: 0",
				SARIF:      []byte("{}"),
				ShouldFail: false,
			}, nil
		},
		func(string, []byte) error { return nil },
		failingWriter{},
	)

	assert.Equal(t, 2, code, "a failure to print the report to stdout is a tool error, even if the scan succeeded")
}

func TestNewRootCmd_VersionIsWired(t *testing.T) {
	assert.Equal(t, version, newRootCmd().Version)
}

func TestNewRootCmd_FlagDefaults(t *testing.T) {
	cmd := newRootCmd()

	cfgFlag := cmd.Flags().Lookup("config")
	assert.NotNil(t, cfgFlag)
	assert.Equal(t, ".pipelineguard.yml", cfgFlag.DefValue)

	sarifFlag := cmd.Flags().Lookup("sarif-output")
	assert.NotNil(t, sarifFlag)
	assert.Equal(t, "pipelineguard-results.sarif", sarifFlag.DefValue)
}
