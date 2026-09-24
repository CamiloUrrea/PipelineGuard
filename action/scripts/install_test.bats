#!/usr/bin/env bats
#
# Tests for install.sh. No network, no GitHub: the helper functions are tested
# by sourcing install.sh (its download flow is guarded by BASH_SOURCE == 0, so
# sourcing never triggers a download), and the few full-flow tests run the
# script as its own process with a FAKE curl first on PATH.

setup() {
	source "${BATS_TEST_DIRNAME}/install.sh"
	# install.sh turns on `set -euo pipefail` when sourced. Drop only -u. Do NOT
	# `set +e`: bats relies on -e so that every assertion line (not just the last
	# one) can fail the test. Functions that are expected to fail are always
	# called through `run`, which captures $status/$output without aborting.
	set +u
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

# --- resolve_version -------------------------------------------------------
#
# A FAKE `curl` is put first on PATH — never the real GitHub API. It logs every
# call to CURL_CALLS_LOG (a spy), answers the Releases API URL with the JSON in
# FAKE_RELEASES_JSON, and fails (like a 404) for anything else. With
# FAKE_CURL_FAIL=1 it fails every call.

install_fake_curl() {
	FAKE_BIN_DIR="${BATS_TEST_TMPDIR}/bin"
	mkdir -p "$FAKE_BIN_DIR"
	export CURL_CALLS_LOG="${BATS_TEST_TMPDIR}/curl_calls.log"
	export FAKE_RELEASES_JSON="${BATS_TEST_TMPDIR}/releases.json"
	: >"$CURL_CALLS_LOG"

	cat >"${FAKE_BIN_DIR}/curl" <<'EOF'
#!/usr/bin/env bash
echo "curl $*" >>"$CURL_CALLS_LOG"
if [[ -n "${FAKE_CURL_FAIL:-}" ]]; then
	echo "curl: (22) The requested URL returned error: 500" >&2
	exit 22
fi
for arg in "$@"; do
	if [[ "$arg" == https://api.github.com/repos/*/releases* ]]; then
		cat "$FAKE_RELEASES_JSON"
		exit 0
	fi
done
echo "curl: (22) The requested URL returned error: 404" >&2
exit 22
EOF
	chmod +x "${FAKE_BIN_DIR}/curl"
	export PATH="${FAKE_BIN_DIR}:${PATH}"
}

# write_releases_json <tag>... writes a Releases API-shaped response (pretty
# printed, nested objects, other *_name fields) listing the given tags.
write_releases_json() {
	local first=1 tag
	{
		echo "["
		for tag in "$@"; do
			[[ $first -eq 1 ]] || echo "  ,"
			first=0
			cat <<EOF
  {
    "url": "https://api.github.com/repos/CamiloUrrea/PipelineGuard/releases/1",
    "tag_name": "${tag}",
    "target_commitish": "main",
    "name": "PipelineGuard ${tag}",
    "draft": false,
    "author": { "login": "github-actions[bot]", "type": "Bot" },
    "assets": [ { "name": "checksums.txt", "label": "" } ]
  }
EOF
		done
		echo "]"
	} >"$FAKE_RELEASES_JSON"
}

@test "resolve_version: v1 -> highest v1.x.y from the API (v1.0.0, v1.2.3, v0.9.0 -> v1.2.3)" {
	install_fake_curl
	write_releases_json "v1.0.0" "v1.2.3" "v0.9.0"

	run resolve_version "v1"
	[ "$status" -eq 0 ]
	[ "${lines[-1]}" = "v1.2.3" ]
	grep -q "api.github.com/repos/CamiloUrrea/PipelineGuard/releases" "$CURL_CALLS_LOG"
}

@test "resolve_version: numeric sort, not lexical (v1.10.0 beats v1.9.0)" {
	install_fake_curl
	write_releases_json "v1.9.0" "v1.10.0" "v1.2.0"

	run resolve_version "v1"
	[ "$status" -eq 0 ]
	[ "${lines[-1]}" = "v1.10.0" ]
}

@test "resolve_version: pre-releases and other majors never match (v1.3.0-rc.1, v10.0.0)" {
	install_fake_curl
	write_releases_json "v1.3.0-rc.1" "v10.0.0" "v1.2.3"

	run resolve_version "v1"
	[ "$status" -eq 0 ]
	[ "${lines[-1]}" = "v1.2.3" ]
}

@test "resolve_version: v1 with no matching release -> clear error, no tag printed" {
	install_fake_curl
	write_releases_json "v0.1.0" "v0.9.0" "v2.0.0"

	run resolve_version "v1"
	[ "$status" -ne 0 ]
	[[ "$output" == *"no release matching 'v1.X.Y'"* ]]
	# Nothing that looks like a usable tag may leak onto stdout.
	! grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$' <<<"$output" || false
}

@test "resolve_version: API request fails -> clear error" {
	install_fake_curl
	export FAKE_CURL_FAIL=1

	run resolve_version "v1"
	[ "$status" -ne 0 ]
	[[ "$output" == *"could not query"* ]]
	[[ "$output" == *"api.github.com"* ]]
}

@test "resolve_version: full version v0.1.0 is returned as-is, curl never called" {
	install_fake_curl
	write_releases_json "v1.2.3"

	run resolve_version "v0.1.0"
	[ "$status" -eq 0 ]
	[ "$output" = "v0.1.0" ]
	# Spy: the fake curl must not have been invoked at all.
	[ ! -s "$CURL_CALLS_LOG" ]
}

@test "resolve_version: GITHUB_TOKEN, when set, is sent as a bearer token" {
	install_fake_curl
	write_releases_json "v1.0.0"
	export GITHUB_TOKEN="fake-token-123"

	run resolve_version "v1"
	[ "$status" -eq 0 ]
	grep -q "Authorization: Bearer fake-token-123" "$CURL_CALLS_LOG"
}

@test "install.sh v1 (script run): downloads from the RESOLVED tag, not from 'v1'" {
	install_fake_curl
	write_releases_json "v1.0.0" "v1.2.3"
	unset GITHUB_PATH

	# Run the script as its own process, with its own set -e, exactly as the
	# Action does (under `run`, a sourced install_pipelineguard would not stop at
	# a failing step). The fake curl 404s every download, so the install stops
	# right after the first download attempt — enough to see which URL it built.
	run bash "${BATS_TEST_DIRNAME}/install.sh" "v1"
	[ "$status" -ne 0 ]
	grep -q "releases/download/v1.2.3/pipelineguard_1.2.3_" "$CURL_CALLS_LOG"
	! grep -q "releases/download/v1/" "$CURL_CALLS_LOG" || false
}

@test "install.sh v1 (script run): no matching release -> fails before any download" {
	install_fake_curl
	write_releases_json "v0.1.0"

	run bash "${BATS_TEST_DIRNAME}/install.sh" "v1"
	[ "$status" -ne 0 ]
	[[ "$output" == *"no release matching"* ]]
	! grep -q "releases/download/" "$CURL_CALLS_LOG" || false
}
