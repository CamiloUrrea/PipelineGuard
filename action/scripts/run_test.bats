#!/usr/bin/env bats
#
# Tests for run.sh. A FAKE `pipelineguard` is placed first on PATH — never the
# real binary. No network, no GitHub.

setup() {
	RUN_SH="${BATS_TEST_DIRNAME}/run.sh"
	FAKE_BIN_DIR="${BATS_TEST_TMPDIR}/bin"
	mkdir -p "$FAKE_BIN_DIR"

	# Each test runs in its own dir so the report file doesn't collide.
	WORK_DIR="${BATS_TEST_TMPDIR}/work"
	mkdir -p "$WORK_DIR"

	export GITHUB_OUTPUT="${BATS_TEST_TMPDIR}/github_output"
	: >"$GITHUB_OUTPUT"
}

# make_fake_pipelineguard <exit_code>
# Writes a stub that echoes a known marker (plus its args) and exits <exit_code>.
make_fake_pipelineguard() {
	local code="$1"
	cat >"${FAKE_BIN_DIR}/pipelineguard" <<EOF
#!/usr/bin/env bash
echo "## PipelineGuard stub report"
echo "args: \$*"
exit ${code}
EOF
	chmod +x "${FAKE_BIN_DIR}/pipelineguard"
}

@test "stub exits 0: captured exit-code 0, report has stub stdout, run.sh exits 0" {
	make_fake_pipelineguard 0
	cd "$WORK_DIR"

	PATH="${FAKE_BIN_DIR}:${PATH}" run bash "$RUN_SH"

	[ "$status" -eq 0 ]
	grep -qx "exit-code=0" "$GITHUB_OUTPUT"
	grep -qx "report-path=pipelineguard-report.md" "$GITHUB_OUTPUT"
	grep -q "PipelineGuard stub report" "${WORK_DIR}/pipelineguard-report.md"
}

@test "stub exits 1: captured exit-code 1, but run.sh still exits 0" {
	make_fake_pipelineguard 1
	cd "$WORK_DIR"

	PATH="${FAKE_BIN_DIR}:${PATH}" run bash "$RUN_SH"

	[ "$status" -eq 0 ]
	grep -qx "exit-code=1" "$GITHUB_OUTPUT"
}

@test "stub exits 2: captured exit-code 2, run.sh still exits 0" {
	make_fake_pipelineguard 2
	cd "$WORK_DIR"

	PATH="${FAKE_BIN_DIR}:${PATH}" run bash "$RUN_SH"

	[ "$status" -eq 0 ]
	grep -qx "exit-code=2" "$GITHUB_OUTPUT"
}

@test "config/sarif env vars are forwarded to the binary" {
	make_fake_pipelineguard 0
	cd "$WORK_DIR"

	CONFIG_PATH="custom.yml" SARIF_OUTPUT="custom.sarif" \
		PATH="${FAKE_BIN_DIR}:${PATH}" run bash "$RUN_SH"

	[ "$status" -eq 0 ]
	grep -q -- "--config custom.yml" "${WORK_DIR}/pipelineguard-report.md"
	grep -q -- "--sarif-output custom.sarif" "${WORK_DIR}/pipelineguard-report.md"
}

@test "pipelineguard absent from PATH: run.sh fails immediately, non-zero, clear message" {
	cd "$WORK_DIR"
	# A minimal PATH that still has coreutils but NO pipelineguard.
	local min_path
	min_path="$(dirname "$(command -v bash)"):$(dirname "$(command -v grep)")"

	PATH="$min_path" run bash "$RUN_SH"

	[ "$status" -ne 0 ]
	[[ "$output" == *"pipelineguard"* ]]
	[[ "$output" == *"not on PATH"* || "$output" == *"not found"* ]]
}
