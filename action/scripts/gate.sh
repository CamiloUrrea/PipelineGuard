#!/usr/bin/env bash
#
# gate.sh — final enforcement step (Bloque 16). Reads the EXIT_CODE captured by
# run.sh (Bloque 13, docs/action-run.md) as a step output and decides the real
# result of the job. This is the ONLY step in the composite action that is
# allowed to fail the job over what pipelineguard found — every step before it
# runs unconditionally so the SARIF upload and PR comment always happen first.
#
# Case "0"      → pipelineguard ran, nothing crossed the policy threshold. Silent.
# Case "1"      → pipelineguard ran, a real security violation was found. Fail loud.
# Case "2"      → pipelineguard itself broke internally. Not a security verdict.
# Anything else → run.sh (or the step wiring around it) never produced one of the
#                 three legitimate codes above. That is a bug in the Action, not
#                 a pipelineguard verdict — see docs/action-gate.md for why this
#                 is treated as an Action failure, not a security failure.
#
# A `case` match is used instead of `exit "$EXIT_CODE"` so an unset or
# non-numeric value (e.g. "abc") can never reach `exit` directly — bash would
# handle that silently/oddly instead of raising a clear error.

set -euo pipefail

main() {
	local exit_code="${EXIT_CODE:-}"

	case "$exit_code" in
	0)
		exit 0
		;;
	1)
		echo "gate.sh: PipelineGuard detected a real security policy violation — failing the build. See the report above for details." >&2
		exit 1
		;;
	2)
		echo "gate.sh: PipelineGuard failed internally (not a security verdict) — see the run step's logs above for the tool error." >&2
		exit 2
		;;
	*)
		echo "gate.sh: EXIT_CODE is empty or not a valid pipelineguard exit code ('${exit_code}') — this is a bug in the Action's own wiring, not a pipelineguard verdict." >&2
		exit 1
		;;
	esac
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	main
fi
