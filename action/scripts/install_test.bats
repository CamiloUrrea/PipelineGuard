#!/usr/bin/env bats
#
# Unit tests for the pure helper functions in install.sh.
# No network, no GitHub: install.sh is sourced (its download flow is guarded by
# BASH_SOURCE == 0, so sourcing never triggers a real download).

setup() {
	source "${BATS_TEST_DIRNAME}/install.sh"
	# install.sh sets `set -e`; relax it so failing assertions don't abort oddly.
	set +e
}

# --- detect_os -------------------------------------------------------------

@test "detect_os: Linux -> linux" {
	uname() { echo "Linux"; }
	run detect_os
	[ "$status" -eq 0 ]
	[ "$output" = "linux" ]
}

@test "detect_os: Darwin -> darwin" {
	uname() { echo "Darwin"; }
	run detect_os
	[ "$status" -eq 0 ]
	[ "$output" = "darwin" ]
}

@test "detect_os: MINGW64_NT -> windows" {
	uname() { echo "MINGW64_NT-10.0-22631"; }
	run detect_os
	[ "$status" -eq 0 ]
	[ "$output" = "windows" ]
}

@test "detect_os: unknown -> error" {
	uname() { echo "Plan9"; }
	run detect_os
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported OS"* ]]
	[[ "$output" == *"Plan9"* ]]
}

# --- detect_arch ---------------------------------------------------------

@test "detect_arch: x86_64 -> amd64" {
	uname() { echo "x86_64"; }
	run detect_arch
	[ "$status" -eq 0 ]
	[ "$output" = "amd64" ]
}

@test "detect_arch: aarch64 -> arm64" {
	uname() { echo "aarch64"; }
	run detect_arch
	[ "$status" -eq 0 ]
	[ "$output" = "arm64" ]
}

@test "detect_arch: arm64 -> arm64" {
	uname() { echo "arm64"; }
	run detect_arch
	[ "$status" -eq 0 ]
	[ "$output" = "arm64" ]
}

@test "detect_arch: unknown -> error, never a silent default" {
	uname() { echo "sparc64"; }
	run detect_arch
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported architecture"* ]]
	[[ "$output" == *"sparc64"* ]]
	[ -z "$(echo "$output" | grep -E '^(amd64|arm64)$')" ]
}

# --- archive_name ------------------------------------------------------------

@test "archive_name: linux amd64 -> tar.gz" {
	run archive_name "1.2.3" "linux" "amd64"
	[ "$status" -eq 0 ]
	[ "$output" = "pipelineguard_1.2.3_linux_amd64.tar.gz" ]
}

@test "archive_name: linux arm64 -> tar.gz" {
	run archive_name "1.2.3" "linux" "arm64"
	[ "$output" = "pipelineguard_1.2.3_linux_arm64.tar.gz" ]
}

@test "archive_name: darwin amd64 -> tar.gz" {
	run archive_name "1.2.3" "darwin" "amd64"
	[ "$output" = "pipelineguard_1.2.3_darwin_amd64.tar.gz" ]
}

@test "archive_name: darwin arm64 -> tar.gz" {
	run archive_name "1.2.3" "darwin" "arm64"
	[ "$output" = "pipelineguard_1.2.3_darwin_arm64.tar.gz" ]
}

@test "archive_name: windows amd64 -> zip" {
	run archive_name "1.2.3" "windows" "amd64"
	[ "$output" = "pipelineguard_1.2.3_windows_amd64.zip" ]
}

@test "archive_name: windows arm64 -> zip" {
	run archive_name "1.2.3" "windows" "arm64"
	[ "$output" = "pipelineguard_1.2.3_windows_arm64.zip" ]
}

# --- verify_checksum -------------------------------------------------------

@test "verify_checksum: matching checksum succeeds with no error output" {
	local dir="$BATS_TEST_TMPDIR"
	local asset="pipelineguard_1.2.3_linux_amd64.tar.gz"
	printf 'downloaded bytes\n' >"${dir}/${asset}"

	local hash
	hash="$(sha256sum "${dir}/${asset}" | awk '{ print $1 }')"
	printf '%s  %s\n' "$hash" "$asset" >"${dir}/checksums.txt"
	printf '%s  %s\n' "deadbeef" "unrelated_file.zip" >>"${dir}/checksums.txt"

	run verify_checksum "${dir}/${asset}" "${dir}/checksums.txt"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "verify_checksum: mismatch fails with a clear message" {
	local dir="$BATS_TEST_TMPDIR"
	local asset="pipelineguard_1.2.3_linux_amd64.tar.gz"
	printf 'the real downloaded bytes\n' >"${dir}/${asset}"

	local wrong="0000000000000000000000000000000000000000000000000000000000000000"
	printf '%s  %s\n' "$wrong" "$asset" >"${dir}/checksums.txt"

	run verify_checksum "${dir}/${asset}" "${dir}/checksums.txt"
	[ "$status" -ne 0 ]
	[[ "$output" == *"checksum mismatch"* ]]
	[[ "$output" == *"$asset"* ]]
}

@test "verify_checksum: no entry for the file fails loudly" {
	local dir="$BATS_TEST_TMPDIR"
	local asset="pipelineguard_1.2.3_linux_amd64.tar.gz"
	printf 'bytes\n' >"${dir}/${asset}"
	printf '%s  %s\n' "abc123" "some_other_asset.tar.gz" >"${dir}/checksums.txt"

	run verify_checksum "${dir}/${asset}" "${dir}/checksums.txt"
	[ "$status" -ne 0 ]
	[[ "$output" == *"no checksum entry"* ]]
}
