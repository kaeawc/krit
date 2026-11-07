#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "=== Krit Regression Check ==="
echo ""

check() {
    local name="$1" path="$2" min="$3" max="$4"
    count=$(./krit --no-fir -f json -no-cache -no-type-inference -no-type-oracle -q "$path" 2>/dev/null | go run ./internal/devtools/jsonstat -mode findings 2>/dev/null)
    if [ "$count" -ge "$min" ] && [ "$count" -le "$max" ]; then
        echo -e "  ${GREEN}PASS${NC} $name: $count findings (expected $min-$max)"
    else
        echo -e "  ${RED}FAIL${NC} $name: $count findings (expected $min-$max)"
        return 1
    fi
}

FAIL=0
# Ranges include detekt-parity default-on rules (e.g. MayBeConstant flags the ~30
# non-const vals in Constants.kt, true positives).
check "playground/webservice" "playground/kotlin-webservice/" 40 80 || FAIL=$((FAIL + 1))
check "playground/android-app" "playground/android-app/" 40 85 || FAIL=$((FAIL + 1))

echo ""
if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All regression checks passed.${NC}"
else
    echo -e "${RED}$FAIL regression check(s) failed.${NC}"
    exit 1
fi
