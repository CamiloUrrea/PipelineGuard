// Package scanners provides the real implementations of
// orchestrator.ScannerFunc: they locate the gitleaks / trivy binary on PATH,
// run it against the current directory, and return the raw JSON report bytes.
//
// IMPORTANT: the "happy path" — a real binary running and producing a valid
// report — is NOT covered by unit tests in this package. That needs the
// binaries installed and is verified manually, not in CI yet. What the tests do
// cover is binary-not-found handling and the isExecutionFailure decision (see
// scanners_test.go and docs/scanners.md).
package scanners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Binary names are package variables so tests can point them at a name that is
// guaranteed not to be on PATH. Production code never reassigns them.
var (
	gitleaksBinary = "gitleaks"
	trivyBinary    = "trivy"
)

// scanTimeout bounds how long a single scanner run may take; a hung scanner is
// killed instead of hanging pipelineguard (and the CI job) indefinitely. It is
// a package variable, like the binary names above, so tests can shrink it.
var scanTimeout = 5 * time.Minute

// scanWaitDelay is how long cmd.Wait keeps waiting for the scanner's I/O after
// the process was killed on timeout. Without it, a child process that inherited
// stderr could keep the pipe open and defeat the timeout.
const scanWaitDelay = 5 * time.Second

// RunGitleaks runs gitleaks against the current directory and returns the raw
// JSON report bytes. It satisfies orchestrator.ScannerFunc.
//
// Command: `gitleaks detect --report-format json --report-path <tmp> --no-banner`
// (matches docs/parsers.md). gitleaks exits non-zero simply because it found
// leaks, so a non-zero exit with a populated report file is treated as success —
// see isExecutionFailure.
func RunGitleaks() ([]byte, error) {
	return runScanner(
		"gitleaks",
		gitleaksBinary,
		"https://github.com/gitleaks/gitleaks",
		func(reportPath string) []string {
			return []string{
				"detect",
				"--report-format", "json",
				"--report-path", reportPath,
				"--no-banner",
			}
		},
	)
}

// RunTrivy runs a trivy filesystem scan against the current directory and
// returns the raw JSON report bytes. It satisfies orchestrator.ScannerFunc.
//
// Command: `trivy fs --format json --output <tmp> .` (matches docs/parsers.md).
// By default trivy exits 0 whether or not it finds vulnerabilities, so any
// non-zero exit here is a real failure unless a report was still produced.
func RunTrivy() ([]byte, error) {
	return runScanner(
		"trivy",
		trivyBinary,
		"https://github.com/aquasecurity/trivy",
		func(reportPath string) []string {
			return []string{
				"fs",
				"--format", "json",
				"--output", reportPath,
				".",
			}
		},
	)
}

// runScanner is the shared body of RunGitleaks/RunTrivy: locate the binary, give
// the scanner a temp file to write its JSON report to, run it, and use
// isExecutionFailure to decide whether a non-zero exit was a real failure or
// just "findings were found".
func runScanner(name, binary, installURL string, buildArgs func(reportPath string) []string) ([]byte, error) {
	if _, err := exec.LookPath(binary); err != nil {
		return nil, fmt.Errorf("%s no encontrado en PATH: instálalo desde %s", name, installURL)
	}

	// Reserve a temp file for the scanner to write its JSON report to. Writing
	// to a file is more reliable than capturing stdout, which can interleave
	// log lines with the JSON.
	reportFile, err := os.CreateTemp("", "pipelineguard-"+name+"-*.json")
	if err != nil {
		return nil, fmt.Errorf("creating temp report file for %s: %w", name, err)
	}
	reportPath := reportFile.Name()
	// The scanner opens reportPath by name; we only needed CreateTemp to
	// reserve it. Close our handle now; always remove the file at the end.
	_ = reportFile.Close()
	defer func() { _ = os.Remove(reportPath) }()

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, binary, buildArgs(reportPath)...)
	cmd.Stderr = &stderr
	cmd.WaitDelay = scanWaitDelay
	runErr := cmd.Run()

	// When the deadline fires, CommandContext kills the process and Run returns
	// a bare "signal: killed" / exit error. Replace it with an explicit timeout
	// error (wrapping context.DeadlineExceeded) so the cause is unambiguous.
	if runErr != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		runErr = fmt.Errorf("timeout: %s did not finish within %s: %w",
			name, scanTimeout, context.DeadlineExceeded)
	}

	reportBytes, readErr := os.ReadFile(reportPath)
	hasContent := readErr == nil && len(strings.TrimSpace(string(reportBytes))) > 0

	if isExecutionFailure(runErr, hasContent) {
		return nil, fmt.Errorf("running %s failed: %w (stderr: %s)",
			name, runErr, strings.TrimSpace(stderr.String()))
	}

	if readErr != nil {
		return nil, fmt.Errorf("reading %s report file: %w", name, readErr)
	}

	return reportBytes, nil
}

// isExecutionFailure separates "the process could not run at all" (a real
// error) from "the process ran and exited non-zero" (which, for a scanner like
// gitleaks, just means it found something — not that anything broke).
//
// The exit-code trap: `exit code != 0` does NOT reliably mean failure.
//   - gitleaks exits 1 when it finds leaks.
//   - trivy exits 0 even when it finds vulnerabilities (unless --exit-code is set,
//     which we don't).
//
// So a non-zero exit is only a real failure when the scanner produced no report
// at all.
//
// Returns true when:
//   - runErr is an *exec.Error (binary missing / not executable / permissions —
//     the process never started), or
//   - runErr is an *exec.ExitError (ran, exited non-zero) AND no report content
//     was produced, or
//   - runErr wraps context.DeadlineExceeded (the scan hit scanTimeout and was
//     killed) — even if a partial report was written, it cannot be trusted, or
//   - runErr is any other non-nil error (command setup, I/O).
//
// Returns false when runErr is nil, or when the process exited non-zero but a
// non-empty report file exists.
func isExecutionFailure(runErr error, reportFileHasContent bool) bool {
	if runErr == nil {
		return false
	}

	if errors.Is(runErr, context.DeadlineExceeded) {
		return true
	}

	var execErr *exec.Error
	if errors.As(runErr, &execErr) {
		return true
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return !reportFileHasContent
	}

	return true
}
