#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "=== Krit Regression Check ==="
echo ""

check() {
    local name="$1" path="$2" min="$3" max="$4"
    count=$(./krit -f json -no-cache -no-type-inference -no-type-oracle -q "$path" 2>/dev/null | python3 -c "import json,sys;print(len(json.load(sys.stdin).get('findings',[])))" 2>/dev/null)
    if [ "$count" -ge "$min" ] && [ "$count" -le "$max" ]; then
        echo -e "  ${GREEN}PASS${NC} $name: $count findings (expected $min-$max)"
    else
        echo -e "  ${RED}FAIL${NC} $name: $count findings (expected $min-$max)"
        return 1
    fi
}

FAIL=0
check "playground/webservice" "playground/kotlin-webservice/" 40 80 || ((FAIL++))
check "playground/android-app" "playground/android-app/" 50 100 || ((FAIL++))

echo ""
if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All regression checks passed.${NC}"
else
    echo -e "${RED}$FAIL regression check(s) failed.${NC}"
    exit 1
fi
