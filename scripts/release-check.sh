#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
PASS=0
FAIL=0

check() {
    local name="$1"
    shift
    echo -n "  $name... "
    if "$@" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASS=$((PASS + 1))
    else
        echo -e "${RED}FAIL${NC}"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Krit Release Checklist ==="
echo ""

echo "Build:"
check "krit binary" go build -ldflags "-s -w" -o krit ./cmd/krit/
check "krit-lsp binary" go build -ldflags "-s -w" -o krit-lsp ./cmd/krit-lsp/
check "krit-mcp binary" go build -ldflags "-s -w" -o krit-mcp ./cmd/krit-mcp/

echo ""
echo "Quality:"
check "go vet" go vet ./...
check "all tests" go test ./... -count=1 -timeout 120s

echo ""
echo "Integration:"
check "playground webservice" bash -c './krit -f json -no-cache -no-type-inference -no-type-oracle -q playground/kotlin-webservice/ > /dev/null 2>&1; [ $? -le 1 ]'
check "playground android-app" bash -c './krit -f json -no-cache -no-type-inference -no-type-oracle -q playground/android-app/ > /dev/null 2>&1; [ $? -le 1 ]'

echo ""
echo "CLI:"
check "--version" ./krit --version
check "--list-rules" ./krit --list-rules
check "--generate-schema" ./krit --generate-schema
check "--completions bash" ./krit --completions bash
check "krit-lsp --version" ./krit-lsp --version
check "krit-mcp --version" ./krit-mcp --version

echo ""
echo "Output formats:"
for fmt in json sarif plain checkstyle; do
    check "$fmt output" bash -c "./krit -f $fmt -no-cache -no-type-inference -no-type-oracle -q playground/kotlin-webservice/ > /dev/null 2>&1; [ \$? -le 1 ]"
done

echo ""
echo "Files:"
check "README.md exists" test -f README.md
check "LICENSE exists" test -f LICENSE
check "CONTRIBUTING.md exists" test -f CONTRIBUTING.md
check "CHANGELOG.md exists" test -f CHANGELOG.md
check ".goreleaser.yml exists" test -f .goreleaser.yml
check "mkdocs.yml exists" test -f mkdocs.yml

echo ""
echo "Git:"
check "working tree clean" git diff --quiet HEAD
check "no untracked Go files" bash -c '[ -z "$(git ls-files --others --exclude-standard *.go)" ]'

echo ""
echo "================================"
echo -e "Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}"

if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo "Fix failures before tagging release."
    exit 1
else
    echo ""
    echo "Ready to release! Run:"
    echo "  git tag v0.1.0"
    echo "  git push origin main --tags"
fi
