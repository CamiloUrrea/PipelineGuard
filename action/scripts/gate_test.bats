#!/usr/bin/env bats
#
# Tests for gate.sh. No network, no GitHub, no pipelineguard binary involved —
# this script only inspects the EXIT_CODE env var and decides exit/stderr.

setup() {
	GATE_SH="${BATS_TEST_DIRNAME}/gate.sh"
}

@test "EXIT_CODE=0: exits 0, no output at all" {
	EXIT_CODE=0 run bash "$GATE_SH"

	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "EXIT_CODE=1: exits 1, message mentions a security violation" {
	EXIT_CODE=1 run bash "$GATE_SH"

	[ "$status" -eq 1 ]
	[[ "$output" == *"security"* ]]
}

@test "EXIT_CODE=2: exits 2, message distinguishes a tool failure from a security verdict" {
	EXIT_CODE=2 run bash "$GATE_SH"

	[ "$status" -eq 2 ]
	[[ "$output" == *"internally"* ]]
	[[ "$output" != *"security policy violation"* ]]
}

@test "EXIT_CODE unset: exits 1, message points at the Action's own wiring" {
	unset EXIT_CODE
	run bash "$GATE_SH"

	[ "$status" -eq 1 ]
	[[ "$output" == *"bug in the Action"* ]]
}

@test "EXIT_CODE empty string: exits 1, message points at the Action's own wiring" {
	EXIT_CODE="" run bash "$GATE_SH"

	[ "$status" -eq 1 ]
	[[ "$output" == *"bug in the Action"* ]]
}

@test "EXIT_CODE=abc (non-numeric): exits 1, never attempts exit \"abc\" directly" {
	EXIT_CODE="abc" run bash "$GATE_SH"

	[ "$status" -eq 1 ]
	[[ "$output" == *"bug in the Action"* ]]
	[[ "$output" == *"abc"* ]]
}

@test "EXIT_CODE=3 (numeric but not a legitimate code): treated as Action wiring failure" {
	EXIT_CODE=3 run bash "$GATE_SH"

	[ "$status" -eq 1 ]
	[[ "$output" == *"bug in the Action"* ]]
}
