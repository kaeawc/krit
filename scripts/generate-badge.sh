#!/usr/bin/env bash
# Generate a krit health badge SVG for a project
# Usage: krit -f json . | scripts/generate-badge.sh > badge.svg

set -euo pipefail

INPUT=$(cat)
FINDINGS=$(echo "$INPUT" | python3 -c "import json,sys;print(len(json.load(sys.stdin).get('findings',[])))" 2>/dev/null || echo "?")
FILES=$(echo "$INPUT" | python3 -c "import json,sys;print(json.load(sys.stdin).get('files',0))" 2>/dev/null || echo "?")

if [ "$FINDINGS" = "0" ]; then
    COLOR="#4c1"
    LABEL="passing"
elif [ "$FINDINGS" -lt 10 ] 2>/dev/null; then
    COLOR="#dfb317"
    LABEL="$FINDINGS issues"
elif [ "$FINDINGS" -lt 50 ] 2>/dev/null; then
    COLOR="#fe7d37"
    LABEL="$FINDINGS issues"
else
    COLOR="#e05d44"
    LABEL="$FINDINGS issues"
fi

cat << SVG
<svg xmlns="http://www.w3.org/2000/svg" width="110" height="20">
  <linearGradient id="a" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <rect rx="3" width="110" height="20" fill="#555"/>
  <rect rx="3" x="40" width="70" height="20" fill="${COLOR}"/>
  <rect rx="3" width="110" height="20" fill="url(#a)"/>
  <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
    <text x="21" y="15" fill="#010101" fill-opacity=".3">krit</text>
    <text x="21" y="14">krit</text>
    <text x="74" y="15" fill="#010101" fill-opacity=".3">${LABEL}</text>
    <text x="74" y="14">${LABEL}</text>
  </g>
</svg>
SVG
