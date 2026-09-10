package scanners

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pipelineguard/internal/orchestrator"
)

// Compile-time check that both real scanners satisfy orchestrator.ScannerFunc.
var (
	_ orchestrator.ScannerFunc = RunGitleaks
	_ orchestrator.ScannerFunc = RunTrivy
)

func TestIsExecutionFailure_NoError(t *testing.T) {
	assert.False(t, isExecutionFailure(nil, false))
	assert.False(t, isExecutionFailure(nil, true))
}

func TestIsExecutionFailure_BinaryNotFound(t *testing.T) {
	notFound := &exec.Error{Name: "gitleaks", Err: exec.ErrNotFound}

	// The process never started: always a real failure, regardless of the file.
	assert.True(t, isExecutionFailure(notFound, false))
	assert.True(t, isExecutionFailure(notFound, true))
}

func TestIsExecutionFailure_NonZeroExitWithReport(t *testing.T) {
	// gitleaks exits 1 because it found leaks, but it still wrote the report.
	exitErr := &exec.ExitError{}

	assert.False(t, isExecutionFailure(exitErr, true),
		"non-zero exit + report content = findings, not a failure")
}

func TestIsExecutionFailure_NonZeroExitWithoutReport(t *testing.T) {
	// Ran, exited non-zero, and produced nothing: a genuine failure.
	exitErr := &exec.ExitError{}

	assert.True(t, isExecutionFailure(exitErr, false),
		"non-zero exit + empty report = real failure")
}

func TestIsExecutionFailure_OtherError(t *testing.T) {
	// Setup / I/O / timeout errors are always failures.
	assert.True(t, isExecutionFailure(errors.New("context deadline exceeded"), true))
	assert.True(t, isExecutionFailure(errors.New("some i/o error"), false))
}

func TestRunGitleaks_BinaryNotInPath(t *testing.T) {
	restore := gitleaksBinary
	gitleaksBinary = "pipelineguard-nonexistent-gitleaks-xyz"
	t.Cleanup(func() { gitleaksBinary = restore })

	out, err := RunGitleaks()

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "gitleaks")
	assert.Contains(t, err.Error(), "PATH")
	assert.Contains(t, err.Error(), "github.com/gitleaks/gitleaks")
}

func TestRunTrivy_BinaryNotInPath(t *testing.T) {
	restore := trivyBinary
	trivyBinary = "pipelineguard-nonexistent-trivy-xyz"
	t.Cleanup(func() { trivyBinary = restore })

	out, err := RunTrivy()

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "trivy")
	assert.Contains(t, err.Error(), "PATH")
	assert.Contains(t, err.Error(), "github.com/aquasecurity/trivy")
}
