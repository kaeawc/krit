#!/usr/bin/env bash
set -euo pipefail

# Builds krit and runs it on itself, producing SARIF output.
# Used by the self-lint GitHub Actions workflow.

go build -o krit ./cmd/krit/
./krit --format sarif . > results.sarif || true

python3 - <<'PY'
import json

path = "results.sarif"
with open(path, "r", encoding="utf-8") as fh:
    report = json.load(fh)

fixture_prefixes = (
    "tests/fixtures/java-android-support/",
)

def is_java_fixture(uri):
    normalized = uri.replace("\\", "/")
    return normalized.startswith("tests/fixtures/") and normalized.endswith(".java")

def result_uri(result):
    try:
        return result["locations"][0]["physicalLocation"]["artifactLocation"]["uri"]
    except (KeyError, IndexError, TypeError):
        return ""

for run in report.get("runs", []):
    results = run.get("results", [])
    filtered = []
    for result in results:
        uri = result_uri(result).replace("\\", "/")
        if uri.startswith(fixture_prefixes) or is_java_fixture(uri):
            continue
        filtered.append(result)
    run["results"] = filtered

with open(path, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2)
    fh.write("\n")
PY
