#!/usr/bin/env bash
#
# install.sh — download, verify, and install the PipelineGuard binary from
# GitHub Releases. Consumed by the composite GitHub Action (action.yml, Bloque 13).
#
# Asset naming follows docs/goreleaser.md:
#   pipelineguard_<version>_<os>_<arch>.tar.gz   (linux, darwin)
#   pipelineguard_<version>_<os>_<arch>.zip      (windows)
#   checksums.txt                                (SHA-256 of every asset)
#
# The helper functions are pure / side-effect-free where possible so they can be
# unit-tested with bats (action/scripts/install_test.bats). The download flow
# only runs when the script is executed directly, never when it is sourced.

set -euo pipefail

REPO="CamiloUrrea/PipelineGuard"
BINARY_NAME="pipelineguard"

# detect_os prints the GoReleaser ".Os" value for the current platform.
detect_os() {
	local uname_s
	uname_s="$(uname -s)"
	case "$uname_s" in
	Linux) echo "linux" ;;
	Darwin) echo "darwin" ;;
	MINGW* | MSYS* | CYGWIN*) echo "windows" ;;
	*)
		echo "install.sh: unsupported OS '${uname_s}'" >&2
		return 1
		;;
	esac
}

# detect_arch prints the GoReleaser ".Arch" value for the current platform.
# An unrecognized architecture is a hard error — never a silent default.
detect_arch() {
	local uname_m
	uname_m="$(uname -m)"
	case "$uname_m" in
	x86_64 | amd64) echo "amd64" ;;
	aarch64 | arm64) echo "arm64" ;;
	*)
		echo "install.sh: unsupported architecture '${uname_m}'" >&2
		return 1
		;;
	esac
}

# archive_name prints the exact release asset name for a version/os/arch.
#   archive_name <version> <os> <arch>
archive_name() {
	local version="$1" os="$2" arch="$3" ext="tar.gz"
	if [[ "$os" == "windows" ]]; then
		ext="zip"
	fi
	echo "${BINARY_NAME}_${version}_${os}_${arch}.${ext}"
}

# sha256_hex prints the lowercase hex SHA-256 of a file, using whichever of
# sha256sum / shasum is available.
sha256_hex() {
	local file="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{ print $1 }'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$file" | awk '{ print $1 }'
	else
		echo "install.sh: need sha256sum or shasum to verify downloads" >&2
		return 1
	fi
}

# verify_checksum checks that <file>'s SHA-256 matches its entry in
# <checksums_file> (GoReleaser format: "<hex>  <filename>"). It exits non-zero
# with a clear message on any mismatch or missing entry — the caller must never
# proceed with an unverified binary.
#   verify_checksum <file> <checksums_file>
verify_checksum() {
	local file="$1" checksums_file="$2"
	local name expected actual
	name="$(basename "$file")"

	expected="$(awk -v want="$name" '$2 == want { print $1 }' "$checksums_file")"
	if [[ -z "$expected" ]]; then
		echo "install.sh: no checksum entry for '${name}' in $(basename "$checksums_file")" >&2
		return 1
	fi

	actual="$(sha256_hex "$file")"
	if [[ "$actual" != "$expected" ]]; then
		echo "install.sh: checksum mismatch for '${name}'" >&2
		echo "  expected: ${expected}" >&2
		echo "  actual:   ${actual}" >&2
		return 1
	fi
}

# require_cmd fails loudly if a required command is missing.
require_cmd() {
	local cmd="$1"
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "install.sh: required command '${cmd}' not found" >&2
		return 1
	fi
}

# download fetches a URL to a local path, failing loudly if curl is missing or
# the request fails (curl -f).
#   download <url> <dest>
download() {
	local url="$1" dest="$2"
	curl -fsSL "$url" -o "$dest"
}

# extract_binary pulls the archive contents into dest_dir. tar.gz for unix,
# zip for windows.
#   extract_binary <archive> <os> <dest_dir>
extract_binary() {
	local archive="$1" os="$2" dest_dir="$3"
	if [[ "$os" == "windows" ]]; then
		require_cmd unzip
		unzip -o -q "$archive" -d "$dest_dir"
	else
		tar -xzf "$archive" -C "$dest_dir"
	fi
}

# install_pipelineguard runs the full flow: resolve the platform, download the
# archive and checksums.txt, verify the checksum, extract the binary, and add
# its directory to $GITHUB_PATH so later workflow steps can call `pipelineguard`.
#   install_pipelineguard <version>   (version keeps the leading 'v', e.g. v1.2.3)
install_pipelineguard() {
	local version="${1:-}"
	if [[ -z "$version" ]]; then
		echo "install.sh: usage: install.sh <version>  (e.g. install.sh v1.2.3)" >&2
		return 1
	fi

	require_cmd curl

	# The release tag keeps the leading 'v'; the asset filename drops it.
	local tag="$version"
	local file_version="${version#v}"

	local os arch asset
	os="$(detect_os)"
	arch="$(detect_arch)"
	asset="$(archive_name "$file_version" "$os" "$arch")"

	local base_url="https://github.com/${REPO}/releases/download/${tag}"
	local work_dir
	work_dir="$(mktemp -d)"

	echo "install.sh: downloading ${asset} (${tag})"
	download "${base_url}/${asset}" "${work_dir}/${asset}"
	download "${base_url}/checksums.txt" "${work_dir}/checksums.txt"

	verify_checksum "${work_dir}/${asset}" "${work_dir}/checksums.txt"
	echo "install.sh: checksum OK"

	local bin_dir="${work_dir}/bin"
	mkdir -p "$bin_dir"
	extract_binary "${work_dir}/${asset}" "$os" "$bin_dir"

	local binary="${bin_dir}/${BINARY_NAME}"
	if [[ "$os" == "windows" ]]; then
		binary="${binary}.exe"
	fi
	chmod +x "$binary" 2>/dev/null || true

	if [[ -n "${GITHUB_PATH:-}" ]]; then
		echo "$bin_dir" >>"$GITHUB_PATH"
		echo "install.sh: added ${bin_dir} to \$GITHUB_PATH"
	else
		echo "install.sh: GITHUB_PATH not set; binary is at ${binary}"
	fi
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	install_pipelineguard "${1:-}"
fi
