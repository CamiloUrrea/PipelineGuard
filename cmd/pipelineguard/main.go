// Command pipelineguard runs the full PipelineGuard flow as a single CLI
// command: load config, run the enabled scanners via the orchestrator, write
// the SARIF report to disk, and print the Markdown report to stdout.
//
// The real logic lives in runApp/exitCode (both dependency-injected and pure),
// so main() and Cobra's RunE are just wiring — see docs/cmd.md.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"pipelineguard/internal/config"
	"pipelineguard/internal/orchestrator"
	"pipelineguard/internal/scanners"
)

// version is injected at build time by GoReleaser
// (-X main.version=... in .goreleaser.yaml); "dev" for local builds.
var version = "dev"

// Exit codes are the binary's contract with CI:
//
//	0 → success, nothing exceeded the configured threshold.
//	1 → success, but policy.ShouldFail said the build must fail (a real security
//	    violation, not a tool error).
//	2 → the tool itself failed (bad config, scanner error, SARIF write error).
//
// exitCode is pure so it can be tested directly.
func exitCode(err error, shouldFail bool) int {
	switch {
	case err != nil:
		return 2
	case shouldFail:
		return 1
	default:
		return 0
	}
}

// runApp executes the whole flow with its dependencies injected, so it can be
// tested without a real config file or real scanners.
//
// Error messages go to stderr; only the Markdown report goes to stdout, because
// that stdout is what a later block will post as a PR comment.
func runApp(
	cfgPath, sarifOutPath string,
	loadConfig func(string) (config.Config, error),
	runOrchestrator func(config.Config) (orchestrator.Result, error),
	writeFile func(string, []byte) error,
	stdout io.Writer,
) int {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipelineguard: loading config %q: %v\n", cfgPath, err)
		return exitCode(err, false)
	}

	result, err := runOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipelineguard: %v\n", err)
		return exitCode(err, false)
	}

	if err := writeFile(sarifOutPath, result.SARIF); err != nil {
		fmt.Fprintf(os.Stderr, "pipelineguard: writing SARIF report to %q: %v\n", sarifOutPath, err)
		return exitCode(err, false)
	}

	if _, err := fmt.Fprintln(stdout, result.Markdown); err != nil {
		fmt.Fprintf(os.Stderr, "pipelineguard: writing report to stdout: %v\n", err)
		return exitCode(err, false)
	}

	if result.ShouldFail {
		fmt.Fprintf(os.Stderr, "pipelineguard: %s\n", result.Reason)
	}

	return exitCode(nil, result.ShouldFail)
}

// realOrchestrator wraps orchestrator.Run with the real scanner functions.
func realOrchestrator(cfg config.Config) (orchestrator.Result, error) {
	return orchestrator.Run(cfg, scanners.RunGitleaks, scanners.RunTrivy)
}

func newRootCmd() *cobra.Command {
	var cfgPath, sarifOutPath string

	cmd := &cobra.Command{
		Use:           "pipelineguard",
		Short:         "Orquesta escáneres de seguridad y consolida sus hallazgos en un reporte unificado",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			writeFile := func(path string, data []byte) error {
				return os.WriteFile(path, data, 0o644)
			}
			code := runApp(cfgPath, sarifOutPath, config.Load, realOrchestrator, writeFile, os.Stdout)
			os.Exit(code)
			return nil
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", ".pipelineguard.yml",
		"ruta al archivo de configuración .pipelineguard.yml")
	cmd.Flags().StringVar(&sarifOutPath, "sarif-output", "pipelineguard-results.sarif",
		"ruta donde se escribe el reporte SARIF")

	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Reached only for flag-parsing / usage errors (RunE calls os.Exit itself).
		fmt.Fprintf(os.Stderr, "pipelineguard: %v\n", err)
		os.Exit(2)
	}
}
