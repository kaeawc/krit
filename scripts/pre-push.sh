#!/usr/bin/env bash
# Fast local gate that mirrors the cheap CI jobs (.github/workflows/ci.yml)
# for the files a branch changes. .githooks/pre-push runs it on every push;
# `make prepush` runs it by hand. Install the hook once per clone (worktrees
# share it; hooks already installed keep running):
#
#   make hooks            # scripts/install-hooks.sh
#
# Checks, each only when the changed files make it relevant:
#
#   merge     origin/main merges cleanly, and the merged tree passes
#             `go build ./...` and `go vet ./...` (two PRs that are green on
#             their own can still break main together)
#   gofmt     gofmt -l on changed .go files
#   build     go build ./...                       (CI: build-verify)
#   vet       go vet ./...; also compiles every _test.go (CI: build-verify)
#   lint      golangci-lint run on changed packages (CI: golangci-lint)
#   rules     make lint-rules                      (CI: build-verify)
#   oraclefp  go run ./internal/devtools/oraclefpcheck (CI: build-verify)
#   schema    schemas/krit-config.schema.json matches `krit --generate-schema`
#   gomod     go mod tidy -diff
#   test      go test on changed packages through scripts/go-test.sh, plus
#             the rule fixture and FIR go-lines tests for fixture changes
#   actions   scripts/lint-actions.sh for .github/ changes (CI: actionlint)
#   docs      scripts/validate_mkdocs_nav.sh, then mkdocs build --strict when
#             mkdocs is installed (Documentation workflow)
#   winvet    GOOS=windows go vet ./... when x86_64-w64-mingw32-gcc is on
#             PATH (CI: windows-vet)
#   warmclean the FIR warm-versus-clean mutation matrix rows (CI:
#             warm-clean-matrix 1/2, ~150 s) for cache, hashing, oracle and
#             FIR changes, when java and a krit-fir jar built from the
#             current sources (make fir-jar) are available (#1708)
#
# JVM, Gradle and Bazel jobs are too slow for a push hook; the script names
# the CI job to watch when those trees change.
#
# Usage: scripts/pre-push.sh [--base REV] [--dry-run]
#
#   --base REV   diff against REV (default: merge-base of HEAD and origin/main)
#   --dry-run    print the checks that would run, one per line (the test
#                check also lists its targets), and exit
#
# Environment:
#   KRIT_PREPUSH_SKIP   comma-separated check names to skip, e.g. "test,lint"
#   KRIT_PREPUSH_FILES  newline-separated changed files (overrides git diff;
#                       used by the planner test)
#   KRIT_PREPUSH_FETCH  0 = don't `git fetch origin main` first (default 1)
#   SKIP_PREPUSH=1 or `git push --no-verify` bypasses the hook entirely.
set -euo pipefail

# --- planning (pure: changed files in, check names out) ---------------------

# changed_packages prints the ./dir package patterns for changed .go files
# that still exist, one per line, sorted and unique.
changed_packages() {
	local f dir
	while IFS= read -r f; do
		case "$f" in
			*.go) ;;
			*) continue ;;
		esac
		case "$f" in
			tests/fixtures/*|*/testdata/*|third_party/*|examples/*) continue ;;
		esac
		dir=$(dirname "$f")
		[ -d "$dir" ] || continue
		printf './%s\n' "$dir"
	done | sort -u
}

has_match() {
	# has_match REGEX: any changed file matches the extended regex.
	grep -Eq -- "$1" <<<"$CHANGED"
}

plan_checks() {
	PLAN=()
	[ -n "$CHANGED" ] || return 0
	local go_changed=0
	if has_match '\.go$|^go\.(mod|sum)$'; then
		go_changed=1
	fi
	GO_CHANGED=$go_changed
	PLAN+=(merge)
	if has_match '\.go$'; then
		PLAN+=(gofmt)
	fi
	if [ "$go_changed" = 1 ]; then
		PLAN+=(build vet lint)
	fi
	if has_match '^internal/(rules|ruleslinter)/'; then
		PLAN+=(rules)
	fi
	if has_match '^internal/(rules|oracle|firchecks|typeinfer)/|^\.krit/oracle-fingerprints\.json$|^playground/'; then
		PLAN+=(oraclefp)
	fi
	if has_match '^internal/(rules|schema|config)/|^schemas/|^cmd/krit/'; then
		PLAN+=(schema)
	fi
	if has_match '^go\.(mod|sum)$'; then
		PLAN+=(gomod)
	fi
	if [ "$go_changed" = 1 ] || has_match '^tests/fixtures/|/testdata/'; then
		PLAN+=(test)
	fi
	if has_match '^\.github/'; then
		PLAN+=(actions)
	fi
	if has_match '^docs/|^mkdocs\.yml$|^(cmd/krit-docgen|internal/docgen)/|^README\.md$|^CONTRIBUTING\.md$'; then
		PLAN+=(docs)
	fi
	if [ "$go_changed" = 1 ] && command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
		PLAN+=(winvet)
	fi
	# Stale-warm cache bugs only show up across the whole shard (timing
	# within the racy-stat window), not in one row (#1708).
	if has_match '^internal/(hashutil|cache|firchecks|oracle|gradlemodel|fsutil|cacheutil)/|^tools/krit-fir/'; then
		PLAN+=(warmclean)
	fi
	local skip name kept=()
	skip=",${KRIT_PREPUSH_SKIP:-},"
	for name in "${PLAN[@]}"; do
		case "$skip" in
			*",$name,"*) ;;
			*) kept+=("$name") ;;
		esac
	done
	PLAN=("${kept[@]+"${kept[@]}"}")
}

# test_targets prints `go test` arguments for the test check, one package
# per line; non-package lines are `-run` filters handled by run_test.
test_targets() {
	changed_packages <<<"$CHANGED"
	if has_match '^tests/fixtures/|^internal/rules/'; then
		echo ./internal/rules
	fi
	# Fixture headers carry the Go rule's expected lines; this test needs no
	# krit-fir jar (CI: test). Its other tests do, so filter it.
	if has_match '^tests/fixtures/|^internal/rules/|^tools/krit-fir/compiler-tests/'; then
		echo 'parity:TestFirGoldenGoLines'
	fi
}

# --- running ----------------------------------------------------------------

FAILED=()
step() { printf '\n==> pre-push: %s\n' "$1"; }
fail() {
	FAILED+=("$1")
	printf 'pre-push: %s FAILED. %s\n' "$1" "$2" >&2
}

need_tool() {
	if ! command -v "$1" >/dev/null 2>&1; then
		fail "$2" "$1 is not on PATH. Install it: $3 (or skip with KRIT_PREPUSH_SKIP=$2)"
		return 1
	fi
}

run_merge() {
	step "merge with origin/main"
	if ! git rev-parse --verify -q origin/main >/dev/null; then
		echo "no origin/main ref; skipping"
		return 0
	fi
	if git merge-base --is-ancestor origin/main HEAD; then
		echo "HEAD already contains origin/main"
		return 0
	fi
	local tmp rc=0
	tmp=$(mktemp -d "${TMPDIR:-/tmp}/krit-prepush.XXXXXX")
	git worktree add -q --detach "$tmp/tree" HEAD
	# The merged tree only ever exists in this throwaway worktree.
	if ! git -C "$tmp/tree" -c user.name=prepush -c user.email=prepush@localhost \
		merge -q --no-edit --no-ff origin/main >/dev/null 2>"$tmp/merge.err"; then
		cat "$tmp/merge.err" >&2
		fail merge "origin/main does not merge cleanly. Rebase: git fetch origin && git rebase origin/main"
		rc=1
	elif [ "${GO_CHANGED:-0}" != 1 ]; then
		echo "merges cleanly"
	elif ! (cd "$tmp/tree" && go build ./... && go vet ./...); then
		fail merge "your branch builds, but not merged with origin/main (another PR changed an API you use). Rebase onto origin/main and fix the build."
		rc=1
	elif has_match '^internal/(rules|schema|config)/|^schemas/|^cmd/krit/' &&
		! (cd "$tmp/tree" && go run ./cmd/krit --generate-schema | diff -q - schemas/krit-config.schema.json >/dev/null); then
		# Generated from every rule's metadata, so a rule change and another
		# PR's schema change can each be fresh alone and stale together.
		fail merge "schemas/krit-config.schema.json is stale once merged with origin/main. Merge or rebase onto origin/main, run make schema, and commit it."
		rc=1
	else
		echo "merged tree builds and vets clean"
	fi
	git worktree remove --force "$tmp/tree" >/dev/null 2>&1 || true
	rm -rf "$tmp"
	return "$rc"
}

run_gofmt() {
	step gofmt
	local files bad
	files=$(grep -E '\.go$' <<<"$CHANGED" | while IFS= read -r f; do [ -f "$f" ] && echo "$f"; done || true)
	[ -n "$files" ] || return 0
	# shellcheck disable=SC2086 # one path per word; repo paths have no spaces
	bad=$(gofmt -l $files)
	if [ -n "$bad" ]; then
		printf '%s\n' "$bad"
		fail gofmt "Fix with: gofmt -w $(tr '\n' ' ' <<<"$bad")"
	fi
}

run_build() {
	step "go build ./..."
	go build ./... || fail build "Fix the compile errors above."
}

run_vet() {
	step "go vet ./..."
	go vet ./... || fail vet "go vet also compiles every _test.go; fix the errors above."
}

run_lint() {
	step "golangci-lint (changed packages)"
	need_tool golangci-lint lint "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" || return 0
	local pkgs
	pkgs=$(changed_packages <<<"$CHANGED")
	[ -n "$pkgs" ] || return 0
	# Whole packages, not --new-from-rev: an edit can orphan a helper on an
	# unchanged line (unused), and CI lints everything.
	# shellcheck disable=SC2086
	golangci-lint run $pkgs || fail lint "Fix the findings above (golangci-lint run ./... is what CI runs)."
}

run_rules() {
	step "make lint-rules"
	make -s lint-rules || fail rules "A rule capability/fix-mode gate failed; see internal/ruleslinter and CLAUDE.md."
}

build_krit() {
	[ -n "${KRIT_PREPUSH_BIN:-}" ] && return 0
	KRIT_PREPUSH_BIN=$(mktemp -d "${TMPDIR:-/tmp}/krit-prepush-bin.XXXXXX")/krit
	go build -o "$KRIT_PREPUSH_BIN" ./cmd/krit/
}

run_oraclefp() {
	step "oracle filter fingerprint gate"
	# oraclefpcheck runs ./krit from the repo root.
	go build -o krit ./cmd/krit/ && go run ./internal/devtools/oraclefpcheck ||
		fail oraclefp "Oracle filter fingerprints changed. If intended: go run ./internal/devtools/oraclefpcheck -update, and commit .krit/oracle-fingerprints.json."
}

run_schema() {
	step "config schema freshness"
	build_krit || { fail schema "krit does not build."; return 0; }
	if ! "$KRIT_PREPUSH_BIN" --generate-schema | diff -q - schemas/krit-config.schema.json >/dev/null; then
		fail schema "schemas/krit-config.schema.json is stale. Regenerate: make schema, and commit it."
	fi
}

run_gomod() {
	step "go mod tidy -diff"
	go mod tidy -diff || fail gomod "Run go mod tidy and commit go.mod/go.sum."
}

run_test() {
	step "go test (changed packages)"
	local targets pkgs=() runs=() t
	targets=$(test_targets | sort -u)
	while IFS= read -r t; do
		[ -n "$t" ] || continue
		case "$t" in
			parity:*) runs+=("${t#parity:}") ;;
			*) [ -n "$(find "${t#./}" -maxdepth 1 -name '*_test.go' -print -quit)" ] && pkgs+=("$t") ;;
		esac
	done <<<"$targets"
	if [ "${#pkgs[@]}" -gt 0 ]; then
		bash scripts/go-test.sh "${pkgs[@]}" || fail test "Fix the failing tests above."
	fi
	for t in "${runs[@]+"${runs[@]}"}"; do
		bash scripts/go-test.sh ./tests/parity -run "^${t}\$" -count=1 ||
			fail test "$t failed; if the Go rule change is intended, regenerate with KRIT_UPDATE_GO_LINES=1."
	done
}

run_actions() {
	step actionlint
	need_tool actionlint actions "go install github.com/rhysd/actionlint/cmd/actionlint@latest" || return 0
	bash scripts/lint-actions.sh || fail actions "Fix the workflow findings above."
}

run_docs() {
	step "docs nav"
	bash scripts/validate_mkdocs_nav.sh || fail docs "A page in the mkdocs.yml nav is missing."
	if command -v mkdocs >/dev/null 2>&1; then
		mkdocs build --strict -q -d "$(mktemp -d)" || fail docs "mkdocs build --strict failed (the Documentation workflow runs this on main)."
	else
		echo "mkdocs not installed; skipping mkdocs build --strict (pip install mkdocs mkdocs-material)"
	fi
}

run_winvet() {
	step "go vet for windows/amd64"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go vet ./... ||
		fail winvet "Unix-only APIs need a build tag or a windows variant."
}

# fir_jar prints a krit-fir jar built from the current sources: KRIT_FIR_JAR
# when set, else make fir-jar's cache entry for this source hash. A jar
# from other sources would test the wrong checker, so none is used then.
fir_jar() {
	if [ -n "${KRIT_FIR_JAR:-}" ]; then
		[ -f "$KRIT_FIR_JAR" ] && printf '%s\n' "$KRIT_FIR_JAR"
		return 0
	fi
	local hash jar
	hash=$(go run ./cmd/krit-dev-jar-hash krit-fir 2>/dev/null) || return 0
	jar="${KRIT_DEV_JAR_DIR:-$HOME/.krit/jars/dev}/$hash/krit-fir.jar"
	[ -f "$jar" ] && printf '%s\n' "$jar"
	return 0
}

kotlin_stdlib_jar() {
	if [ -n "${KOTLIN_STDLIB_JAR:-}" ]; then
		printf '%s\n' "$KOTLIN_STDLIB_JAR"
		return 0
	fi
	local f found=()
	for f in "$HOME"/.gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib/*/*/kotlin-stdlib-*.jar; do
		[ -f "$f" ] || continue
		case "$f" in *sources*|*javadoc*) continue ;; esac
		found+=("$f")
	done
	[ "${#found[@]}" -gt 0 ] && printf '%s\n' "${found[@]}" | sort | tail -1
	return 0
}

run_warmclean() {
	step "warm-versus-clean FIR matrix (CI: warm-clean-matrix 1/2)"
	local jar stdlib
	jar=$(fir_jar)
	stdlib=$(kotlin_stdlib_jar)
	if ! command -v java >/dev/null 2>&1 || [ -z "$jar" ] || [ -z "$stdlib" ]; then
		echo "skipped: needs java, a krit-fir jar for these sources (make fir-jar) and kotlin-stdlib in the Gradle cache. CI's warm-clean-matrix job runs it."
		return 0
	fi
	KRIT_WARM_CLEAN_MATRIX=1 KRIT_FIR_JAR="$jar" KOTLIN_STDLIB_JAR="$stdlib" \
		bash scripts/go-test.sh ./tests/parity/ -count=1 -timeout 30m -run 'TestWarmCleanMutationFIR//^abs$' ||
		fail warmclean "A warm (cached/daemon) result differs from a clean run: a cache key misses an input. See tests/parity/warm_clean_*_test.go."
}

jvm_notes() {
	local note=()
	has_match '^krit-gradle-plugin/' && note+=("krit-gradle-plugin: ./gradlew -p krit-gradle-plugin test (CI: krit-gradle-plugin)")
	has_match '^krit-settings-plugin/' && note+=("krit-settings-plugin: ./gradlew -p krit-settings-plugin test (CI: krit-settings-plugin)")
	has_match '^krit-custom-rule-plugin/' && note+=("krit-custom-rule-plugin: CI job krit-custom-rule-plugin")
	has_match '^tools/krit-fir/' && note+=("krit-fir: CI jobs krit-fir (*) and oracle-backend-parity")
	has_match '^bazel/' && note+=("bazel: CI job bazel-e2e (scripts/bazel-e2e-local.sh)")
	has_match '^tests/fixtures/' && note+=("fixtures: CI job krit-fir (compiler-tests) runs FixtureParityTest against them")
	[ "${#note[@]}" -gt 0 ] || return 0
	printf '\npre-push: not checked locally (JVM/Bazel), watch these CI jobs:\n'
	printf '  - %s\n' "${note[@]}"
}

main() {
	local base="" dry_run=0
	while [ $# -gt 0 ]; do
		case "$1" in
			--base) base=$2; shift 2 ;;
			--dry-run) dry_run=1; shift ;;
			-h|--help) sed -n '2,45p' "$0"; return 0 ;;
			*) echo "unknown argument: $1" >&2; return 2 ;;
		esac
	done
	# Hooks can inherit GIT_DIR/GIT_WORK_TREE; with them set, `git -C` on the
	# throwaway merge worktree would act on this checkout instead.
	unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE
	cd "$(git rev-parse --show-toplevel)"
	if [ -n "${KRIT_PREPUSH_FILES+x}" ]; then
		CHANGED=$KRIT_PREPUSH_FILES
	else
		if [ "${KRIT_PREPUSH_FETCH:-1}" = 1 ]; then
			git fetch -q origin main 2>/dev/null || echo "pre-push: git fetch origin main failed; using the local origin/main" >&2
		fi
		if [ -z "$base" ]; then
			base=$(git merge-base HEAD origin/main 2>/dev/null || git merge-base HEAD main)
		fi
		CHANGED=$(git diff --name-only "$base" HEAD)
		if ! git diff --quiet HEAD -- 2>/dev/null; then
			echo "pre-push: warning: uncommitted changes; checks run on the working tree, not the pushed commit" >&2
		fi
	fi
	CHANGED=$(grep -v '^$' <<<"$CHANGED" || true)
	plan_checks
	if [ "$dry_run" = 1 ]; then
		local name
		for name in "${PLAN[@]+"${PLAN[@]}"}"; do
			if [ "$name" = test ]; then
				printf 'test %s\n' "$(test_targets | sort -u | tr '\n' ' ' | sed 's/ $//')"
			else
				printf '%s\n' "$name"
			fi
		done
		return 0
	fi
	if [ "${#PLAN[@]}" -eq 0 ]; then
		echo "pre-push: nothing to check"
		return 0
	fi
	local start check
	start=$(date +%s)
	for check in "${PLAN[@]}"; do
		"run_$check" || true
	done
	jvm_notes
	if [ -n "${KRIT_PREPUSH_BIN:-}" ]; then
		rm -rf "$(dirname "$KRIT_PREPUSH_BIN")"
	fi
	if [ "${#FAILED[@]}" -gt 0 ]; then
		printf '\npre-push: FAILED: %s (%ss). Fix them, or bypass once with git push --no-verify.\n' "${FAILED[*]}" "$(( $(date +%s) - start ))" >&2
		return 1
	fi
	printf '\npre-push: OK: %s (%ss)\n' "${PLAN[*]}" "$(( $(date +%s) - start ))"
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
	main "$@"
fi
