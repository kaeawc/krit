#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
PASS=0
FAIL=0

run_test() {
    local name="$1"
    shift
    echo -n "  ${name}... "
    local log
    log=$(mktemp)
    if "$@" > "$log" 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASS=$((PASS + 1))
        rm -f "$log"
    else
        echo -e "${RED}FAIL${NC}"
        FAIL=$((FAIL + 1))
        echo "    --- output ---"
        # The tail alone can hide which package failed in a long run. Keep
        # each failing test's own messages (`file_test.go:N: ...`) too:
        # without them a flaky failure cannot be told apart (#954).
        grep -E '^([[:space:]]*--- FAIL|FAIL|panic:)|^[[:space:]]+[[:alnum:]_./-]+_test\.go:[0-9]+:' "$log" | sed 's/^/    /' | head -80 || true
        echo "    ..."
        sed 's/^/    /' "$log" | tail -30
        echo "    --- end ---"
        rm -f "$log"
    fi
}

# For lint commands: exit 0 (no findings) or 1 (findings exist) are both OK.
# Only exit >= 2 indicates an actual error.
run_lint_test() {
    local name="$1"
    shift
    echo -n "  ${name}... "
    local rc=0
    "$@" > /dev/null 2>&1 || rc=$?
    if [ "$rc" -le 1 ]; then
        echo -e "${GREEN}PASS${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "${RED}FAIL${NC} (exit $rc)"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Building binaries ==="
go build -o krit ./cmd/krit/
go build -o krit-lsp ./cmd/krit-lsp/
go build -o krit-mcp ./cmd/krit-mcp/

echo ""
echo "=== Playground Analysis ==="
run_lint_test "kotlin-webservice lint" ./krit --no-fir -f json -no-type-inference -no-type-oracle -q playground/kotlin-webservice/
run_lint_test "android-app lint" ./krit --no-fir -f json -no-type-inference -no-type-oracle -q playground/android-app/
run_lint_test "kotlin-webservice fix (dry-run)" ./krit --no-fir --fix --dry-run -q playground/kotlin-webservice/
run_lint_test "android-app fix (dry-run)" ./krit --no-fir --fix --dry-run -q playground/android-app/
run_lint_test "android-app binary fix (dry-run)" ./krit --no-fir --fix-binary --dry-run -q playground/android-app/

echo ""
echo "=== Diff Mode ==="
FIRST_COMMIT=$(git rev-list --max-parents=0 HEAD)
run_lint_test "diff vs initial commit" ./krit --no-fir --diff "$FIRST_COMMIT" -f json -no-type-inference -no-type-oracle -q playground/kotlin-webservice/

echo ""
echo "=== SARIF Output ==="
run_lint_test "SARIF generation" ./krit --no-fir -f sarif -no-type-inference -no-type-oracle -q -o /tmp/krit-test.sarif playground/kotlin-webservice/

if [ "${KRIT_INTEGRATION_SKIP_GO_TESTS:-0}" = 1 ]; then
    # CI's `test` job already runs every Go test package (no build tags, so
    # this script's cmd/krit, cmd/krit-lsp, and cmd/krit-mcp runs below are a
    # subset of it). Still compile, vet, and link every test package so this
    # job's Go build cache, which the other Linux jobs restore, stays
    # complete. `-exec true` hands each linked test binary to true(1), so no
    # test or TestMain runs.
    echo ""
    echo "=== Go Tests (compile only; KRIT_INTEGRATION_SKIP_GO_TESTS=1) ==="
    run_test "Compile all test packages" bash scripts/go-test.sh ./... -exec true
else
    echo ""
    echo "=== Go Integration Tests ==="
    run_test "CLI tests" go test ./cmd/krit/ -count=1 -timeout 600s
    run_test "LSP tests" go test ./cmd/krit-lsp/ -count=1 -timeout 60s
    run_test "MCP tests" go test ./cmd/krit-mcp/ -count=1 -timeout 60s

    echo ""
    echo "=== Unit Tests ==="
    run_test "All packages" bash scripts/go-test.sh ./... -count=1 -timeout 600s
fi

echo ""
echo "================================"
echo -e "Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}"
if [ "$FAIL" -gt 0 ]; then exit 1; fi
