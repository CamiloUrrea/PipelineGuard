package scanners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pipelineguard/internal/orchestrator"
)

// fakeScannerEnv, when set, turns the test binary itself into a fake scanner
// that just sleeps (see TestMain). The timeout tests point gitleaksBinary /
// trivyBinary at os.Executable() and set this variable, so the "hung scanner"
// is a real child process on every OS — no shell, no `sleep` binary needed.
const fakeScannerEnv = "PIPELINEGUARD_FAKE_SCANNER_SLEEP"

func TestMain(m *testing.M) {
	if d, err := time.ParseDuration(os.Getenv(fakeScannerEnv)); err == nil {
		time.Sleep(d)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

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

func TestIsExecutionFailure_Timeout(t *testing.T) {
	timeoutErr := fmt.Errorf("timeout: gitleaks did not finish within 10ms: %w", context.DeadlineExceeded)

	// A killed-on-timeout scan is always a failure, even if a partial report
	// was written before the kill.
	assert.True(t, isExecutionFailure(timeoutErr, true))
	assert.True(t, isExecutionFailure(timeoutErr, false))
}

// useHangingScanner points *binary at the test executable, which (via TestMain
// and fakeScannerEnv) sleeps for sleepFor, and shrinks scanTimeout to timeout.
func useHangingScanner(t *testing.T, binary *string, sleepFor, timeout time.Duration) {
	t.Helper()

	self, err := os.Executable()
	require.NoError(t, err)

	restoreBinary, restoreTimeout := *binary, scanTimeout
	*binary = self
	scanTimeout = timeout
	t.Cleanup(func() {
		*binary = restoreBinary
		scanTimeout = restoreTimeout
	})
	t.Setenv(fakeScannerEnv, sleepFor.String())
}

func TestRunGitleaks_Timeout(t *testing.T) {
	const sleepFor = 30 * time.Second
	useHangingScanner(t, &gitleaksBinary, sleepFor, 10*time.Millisecond)

	start := time.Now()
	out, err := RunGitleaks()
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timeout")
	assert.Contains(t, err.Error(), "gitleaks")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, sleepFor/2, "RunGitleaks must return on timeout, not wait for the scanner")
}

func TestRunTrivy_Timeout(t *testing.T) {
	const sleepFor = 30 * time.Second
	useHangingScanner(t, &trivyBinary, sleepFor, 10*time.Millisecond)

	start := time.Now()
	out, err := RunTrivy()
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timeout")
	assert.Contains(t, err.Error(), "trivy")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, sleepFor/2, "RunTrivy must return on timeout, not wait for the scanner")
}
