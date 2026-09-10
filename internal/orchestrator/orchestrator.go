// Package orchestrator wires the individual pieces of PipelineGuard together:
// it runs the enabled scanners, normalizes and merges their findings, computes
// the risk score, renders the Markdown and SARIF reports, and asks
// internal/policy whether the build should fail.
//
// Scanners are passed in as functions rather than executed here on purpose: it
// keeps the whole connection flow testable without gitleaks/trivy installed.
// Actually shelling out to the real binaries is the job of cmd/pipelineguard.
package orchestrator

import (
	"fmt"

	"pipelineguard/internal/config"
	"pipelineguard/internal/parsers"
	"pipelineguard/internal/policy"
	"pipelineguard/internal/report"
	"pipelineguard/internal/scoring"
)

// ScannerFunc produces the raw report bytes of a single scanner (the same bytes
// its CLI would write to stdout). An error means the scan itself failed and must
// be surfaced loudly — it is never equivalent to "no findings".
type ScannerFunc func() ([]byte, error)

// Result is everything a PipelineGuard run produces.
type Result struct {
	Findings   []parsers.Finding
	Score      int
	Counts     map[string]int
	Markdown   string
	SARIF      []byte
	ShouldFail bool
	Reason     string
}

// Run executes the enabled scanners and assembles the full Result.
//
// For each scanner: if its flag in cfg.Scanners is false the corresponding
// ScannerFunc is never called. If it is true and the ScannerFunc returns an
// error, or its output fails to parse, Run returns that error wrapped with
// context — it never swallows a scan failure as an empty result.
//
// cfg.Scanners.Semgrep is ignored entirely in this block (not implemented yet).
func Run(cfg config.Config, runGitleaks, runTrivy ScannerFunc) (Result, error) {
	var findings []parsers.Finding

	if cfg.Scanners.Gitleaks {
		raw, err := runGitleaks()
		if err != nil {
			return Result{}, fmt.Errorf("running gitleaks scanner: %w", err)
		}
		parsed, err := parsers.ParseGitleaks(raw)
		if err != nil {
			return Result{}, fmt.Errorf("parsing gitleaks output: %w", err)
		}
		findings = append(findings, parsed...)
	}

	if cfg.Scanners.Trivy {
		raw, err := runTrivy()
		if err != nil {
			return Result{}, fmt.Errorf("running trivy scanner: %w", err)
		}
		parsed, err := parsers.ParseTrivy(raw)
		if err != nil {
			return Result{}, fmt.Errorf("parsing trivy output: %w", err)
		}
		findings = append(findings, parsed...)
	}

	score := scoring.ComputeScore(findings, cfg.SeverityWeights)
	counts := scoring.CountBySeverity(findings)
	markdown := report.GenerateMarkdown(findings, score, counts)

	sarifBytes, err := report.GenerateSARIF(findings)
	if err != nil {
		return Result{}, fmt.Errorf("generating SARIF report: %w", err)
	}

	shouldFail, reason := policy.ShouldFail(findings, cfg)

	return Result{
		Findings:   findings,
		Score:      score,
		Counts:     counts,
		Markdown:   markdown,
		SARIF:      sarifBytes,
		ShouldFail: shouldFail,
		Reason:     reason,
	}, nil
}
