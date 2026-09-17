#!/usr/bin/env bash
#
# comment.sh — post (or update) PipelineGuard's Markdown report as a single
# comment on the current pull request. Consumed by the composite action
# (action.yml).
#
# Every comment we post starts with a fixed hidden marker
# (<!-- pipelineguard-report -->). Before posting we look for an existing comment
# carrying that marker: if found we PATCH it, otherwise we POST a new one. This
# keeps exactly one PipelineGuard comment per PR, updated in place across runs.
#
# The helper functions are pure and testable with bats; the GitHub calls only
# happen when the script is executed directly, not when it is sourced.

set -euo pipefail

MARKER="<!-- pipelineguard-report -->"

# require_cmd fails loudly if a required command is missing.
# Local copy — intentionally duplicated from install.sh / run.sh, not shared.
require_cmd() {
	local cmd="$1"
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "comment.sh: required command '${cmd}' not found" >&2
		return 1
	fi
}

# build_comment_body <marker> <report_path>
# Echoes the marker line, a blank line, then the report file contents.
build_comment_body() {
	local marker="$1" report_path="$2"
	printf '%s\n\n' "$marker"
	cat "$report_path"
}

# find_comment_id <comments_json> <marker>
#   comments_json: a JSON array of {id, body} objects — or the several
#     concatenated arrays that `gh api --paginate` emits. Pass "-" to read it
#     from stdin instead.
#   Echoes the id of the first comment whose body contains <marker>, or nothing
#   if there is no match (including an empty "[]" array).
find_comment_id() {
	local comments_json="$1" marker="$2"
	if [[ "$comments_json" == "-" ]]; then
		comments_json="$(cat)"
	fi
	jq -rs --arg marker "$marker" \
		'[ .[] | .[]? | select((.body // "") | contains($marker)) | .id ] | first // empty' \
		<<<"$comments_json"
}

main() {
	require_cmd gh
	require_cmd jq

	if [[ -z "${PR_NUMBER:-}" ]]; then
		echo "comment.sh: PR_NUMBER is not set — cannot post a PR comment" >&2
		exit 1
	fi

	local report_path="${REPORT_PATH:-pipelineguard-report.md}"
	if [[ ! -f "$report_path" ]]; then
		echo "comment.sh: report file '${report_path}' not found" >&2
		exit 1
	fi

	# gh resolves the {owner}/{repo} placeholders from GH_REPO; fall back to the
	# GITHUB_REPOSITORY that GitHub Actions always sets.
	export GH_REPO="${GH_REPO:-${GITHUB_REPOSITORY:-}}"

	local body_file
	body_file="$(mktemp)"
	build_comment_body "$MARKER" "$report_path" >"$body_file"

	local comments_json existing_id
	comments_json="$(gh api "repos/{owner}/{repo}/issues/${PR_NUMBER}/comments" --paginate)"
	existing_id="$(find_comment_id "$comments_json" "$MARKER")"

	if [[ -n "$existing_id" ]]; then
		echo "comment.sh: updating existing comment ${existing_id}"
		gh api "repos/{owner}/{repo}/issues/comments/${existing_id}" \
			-X PATCH -F "body=@${body_file}"
	else
		echo "comment.sh: creating a new comment on PR #${PR_NUMBER}"
		gh api "repos/{owner}/{repo}/issues/${PR_NUMBER}/comments" \
			-X POST -F "body=@${body_file}"
	fi

	rm -f "$body_file"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	main
fi
