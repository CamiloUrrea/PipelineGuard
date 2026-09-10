#!/usr/bin/env bash
#
# run.sh — execute the pipelineguard binary (installed by install.sh, Bloque 12)
# and expose its exit code as a GitHub Actions step output INSTEAD of failing the
# step. Consumed by the composite action (action.yml).
#
# Why this script does not fail on a non-zero pipelineguard exit: see
# docs/action-run.md. In short — later steps (SARIF upload, PR comment) still
# need to run, and the "does the whole job fail?" decision is taken by a later
# step that reads the saved exit-code output. The ONE hard failure here is
# pipelineguard missing from PATH: there is nothing to defer, nothing to run.

set -euo pipefail

REPORT_PATH="pipelineguard-report.md"

# require_cmd fails loudly if a required command is missing.
# Local copy — intentionally duplicated from install.sh, not shared.
require_cmd() {
	local cmd="$1"
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "run.sh: required command '${cmd}' not found" >&2
		return 1
	fi
}

# run_pipelineguard executes the binary, sends its stdout (the Markdown report)
# to report_path, and prints the binary's exit code on stdout WITHOUT letting
# `set -e` abort this script. The caller captures that number.
#   run_pipelineguard <config_path> <sarif_output> <report_path>
run_pipelineguard() {
	local config_path="$1" sarif_output="$2" report_path="$3"
	local exit_code=0

	pipelineguard --config "$config_path" --sarif-output "$sarif_output" \
		>"$report_path" || exit_code=$?

	echo "$exit_code"
}

main() {
	require_cmd pipelineguard || {
		echo "run.sh: pipelineguard is not on PATH — the install step must run first" >&2
		exit 1
	}

	local config_path="${CONFIG_PATH:-.pipelineguard.yml}"
	local sarif_output="${SARIF_OUTPUT:-pipelineguard-results.sarif}"

	local exit_code
	exit_code="$(run_pipelineguard "$config_path" "$sarif_output" "$REPORT_PATH")"

	echo "run.sh: pipelineguard exited with code ${exit_code}" >&2

	if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
		{
			echo "exit-code=${exit_code}"
			echo "report-path=${REPORT_PATH}"
		} >>"$GITHUB_OUTPUT"
	fi

	# The script itself always succeeds (except the missing-binary case above).
	# The real result of the analysis lives in the exit-code output.
	exit 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	main
fi
