#!/usr/bin/env bats
#
# Tests for comment.sh. A FAKE `gh` is placed first on PATH — never the real gh,
# never the network. `jq` is real (the functions genuinely use it).

setup() {
	COMMENT_SH="${BATS_TEST_DIRNAME}/comment.sh"

	FAKE_BIN_DIR="${BATS_TEST_TMPDIR}/bin"
	mkdir -p "$FAKE_BIN_DIR"

	WORK_DIR="${BATS_TEST_TMPDIR}/work"
	mkdir -p "$WORK_DIR"

	# Where the fake gh records every invocation, and what it returns for the
	# "list comments" call.
	export GH_CALLS_LOG="${BATS_TEST_TMPDIR}/gh_calls.log"
	export GH_FAKE_COMMENTS_FILE="${BATS_TEST_TMPDIR}/comments.json"
	: >"$GH_CALLS_LOG"

	# A report fixture.
	REPORT_FIXTURE="${WORK_DIR}/pipelineguard-report.md"
	printf '## 🛡️ PipelineGuard — Risk Score: 7\n\nSOME-REPORT-CONTENT\n' >"$REPORT_FIXTURE"

	install_fake_gh

	# For the sourced-function tests, load comment.sh (guard stops main()).
	source "$COMMENT_SH"
	set +e
}

install_fake_gh() {
	cat >"${FAKE_BIN_DIR}/gh" <<'EOF'
#!/usr/bin/env bash
echo "gh $*" >>"$GH_CALLS_LOG"
for arg in "$@"; do
	if [[ "$arg" == "-X" ]]; then
		# A write call (PATCH/POST): emit a minimal fake API response.
		echo '{"id": 999, "html_url": "https://example.test/c/999"}'
		exit 0
	fi
done
# Otherwise it is the "list comments" call.
cat "$GH_FAKE_COMMENTS_FILE"
EOF
	chmod +x "${FAKE_BIN_DIR}/gh"
}

# run_comment_sh runs the script directly with the fake gh on PATH.
run_comment_sh() {
	PATH="${FAKE_BIN_DIR}:${PATH}" \
		REPORT_PATH="$REPORT_FIXTURE" \
		GITHUB_REPOSITORY="acme/widgets" \
		run bash "$COMMENT_SH"
}

# --- find_comment_id ------------------------------------------------------

@test "find_comment_id: match -> returns that comment's id" {
	local json='[{"id":11,"body":"hi"},{"id":22,"body":"x <!-- pipelineguard-report --> y"},{"id":33,"body":"bye"}]'
	run find_comment_id "$json" "<!-- pipelineguard-report -->"
	[ "$status" -eq 0 ]
	[ "$output" = "22" ]
}

@test "find_comment_id: no comment carries the marker -> empty" {
	local json='[{"id":11,"body":"hi"},{"id":33,"body":"bye"}]'
	run find_comment_id "$json" "<!-- pipelineguard-report -->"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "find_comment_id: empty array -> empty, no error" {
	run find_comment_id '[]' "<!-- pipelineguard-report -->"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "find_comment_id: handles gh --paginate concatenated arrays" {
	local json='[{"id":1,"body":"a"}][{"id":2,"body":"m <!-- pipelineguard-report -->"}]'
	run find_comment_id "$json" "<!-- pipelineguard-report -->"
	[ "$status" -eq 0 ]
	[ "$output" = "2" ]
}

@test "find_comment_id: reads JSON from stdin when first arg is '-'" {
	run bash -c 'source "'"$COMMENT_SH"'"; echo "[{\"id\":77,\"body\":\"z <!-- pipelineguard-report -->\"}]" | find_comment_id - "<!-- pipelineguard-report -->"'
	[ "$status" -eq 0 ]
	[ "$output" = "77" ]
}

# --- build_comment_body --------------------------------------------------

@test "build_comment_body: starts with the marker and contains the report" {
	run build_comment_body "<!-- pipelineguard-report -->" "$REPORT_FIXTURE"
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "<!-- pipelineguard-report -->" ]]
	[[ "$output" == *"SOME-REPORT-CONTENT"* ]]
	[[ "$output" == *"Risk Score: 7"* ]]
}

# --- full flow ---------------------------------------------------------------

@test "flow: existing marker comment -> PATCH, not POST" {
	printf '%s\n' '[{"id":4242,"body":"old <!-- pipelineguard-report --> report"}]' >"$GH_FAKE_COMMENTS_FILE"

	PR_NUMBER=5 run_comment_sh
	[ "$status" -eq 0 ]

	grep -q -- "-X PATCH" "$GH_CALLS_LOG"
	grep -q "issues/comments/4242" "$GH_CALLS_LOG"
	! grep -q -- "-X POST" "$GH_CALLS_LOG"
}

@test "flow: no existing comment -> POST, not PATCH" {
	printf '%s\n' '[]' >"$GH_FAKE_COMMENTS_FILE"

	PR_NUMBER=5 run_comment_sh
	[ "$status" -eq 0 ]

	grep -q -- "-X POST" "$GH_CALLS_LOG"
	grep -q "issues/5/comments" "$GH_CALLS_LOG"
	! grep -q -- "-X PATCH" "$GH_CALLS_LOG"
}

@test "flow: PR_NUMBER unset -> immediate clear error, gh never called" {
	printf '%s\n' '[]' >"$GH_FAKE_COMMENTS_FILE"

	PATH="${FAKE_BIN_DIR}:${PATH}" REPORT_PATH="$REPORT_FIXTURE" \
		GITHUB_REPOSITORY="acme/widgets" run bash "$COMMENT_SH"

	[ "$status" -ne 0 ]
	[[ "$output" == *"PR_NUMBER"* ]]
	[ ! -s "$GH_CALLS_LOG" ]
}

@test "flow: report file missing -> clear error before any gh write" {
	printf '%s\n' '[]' >"$GH_FAKE_COMMENTS_FILE"

	PATH="${FAKE_BIN_DIR}:${PATH}" REPORT_PATH="${WORK_DIR}/nope.md" \
		PR_NUMBER=5 GITHUB_REPOSITORY="acme/widgets" run bash "$COMMENT_SH"

	[ "$status" -ne 0 ]
	[[ "$output" == *"not found"* ]]
	! grep -q -- "-X" "$GH_CALLS_LOG"
}
