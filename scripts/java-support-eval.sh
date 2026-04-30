#!/usr/bin/env bash
set -euo pipefail

target="${1:-tests/fixtures/java-android-support/mixed-app}"
out="${2:-${TMPDIR:-/tmp}/krit-java-support.json}"
krit_bin="${KRIT_BIN:-./krit}"

if [[ ! -x "$krit_bin" ]]; then
  go build -o "$krit_bin" ./cmd/krit/
fi

"$krit_bin" -no-cache -no-type-inference -no-type-oracle -perf -perf-rules -all-rules -f json -q -o "$out" "$target" || true

python3 - "$target" "$out" <<'PY'
import json
import os
import sys
from collections import Counter

target, out = sys.argv[1], sys.argv[2]
inputs = Counter()
for root, dirs, files in os.walk(target):
    dirs[:] = [d for d in dirs if d not in {".git", ".gradle", "build"}]
    for name in files:
        path = os.path.join(root, name)
        slash = path.replace(os.sep, "/")
        if name.endswith(".java"):
            inputs["java"] += 1
        elif name.endswith(".kt"):
            inputs["kotlin"] += 1
        elif name.endswith((".gradle", ".gradle.kts")) or name == "settings.gradle.kts":
            inputs["gradle"] += 1
        elif name == "AndroidManifest.xml":
            inputs["manifest"] += 1
        elif "/res/" in slash and name.endswith(".xml"):
            inputs["resources"] += 1

with open(out, "r", encoding="utf-8") as fh:
    report = json.load(fh)

java_findings = Counter()
for finding in report.get("findings", []):
    if str(finding.get("file", "")).endswith(".java"):
        java_findings[finding.get("rule", "<unknown>")] += 1

def perf_paths(entries, prefix=""):
    paths = []
    for entry in entries:
        name = entry.get("name", "")
        path = f"{prefix}/{name}" if prefix else name
        if "java" in path.lower() or name == "javaSemanticFacts":
            paths.append(path)
        paths.extend(perf_paths(entry.get("children", []), path))
    return paths

perf_names = perf_paths(report.get("perfTiming", []))

summary = {
    "target": target,
    "report": out,
    "inputs": dict(sorted(inputs.items())),
    "totalFindings": report.get("summary", {}).get("total", 0),
    "javaFindingsByRule": dict(sorted(java_findings.items())),
    "javaPerfEntries": perf_names,
}
print(json.dumps(summary, indent=2, sort_keys=True))
PY
