#!/usr/bin/env python3
"""Oracle filter input-set fingerprint CI gate (issue #333).

Runs `krit --oracle-filter-fingerprint` against each checked-in
playground corpus and compares the emitted fingerprint to
`.krit/oracle-fingerprints.json`. Non-zero exit on drift.

Usage:
    python3 tools/oracle_fingerprint_check.py          # check
    python3 tools/oracle_fingerprint_check.py --update # rewrite baseline

The baseline is keyed by (repo, rule-set). A drift means some rule's
NeedsOracle declaration or oracle filter narrowing has changed
without a corresponding baseline update — i.e. the oracle input set
silently shifted (the Scenario-B-style delta #317 shipped the
fingerprint emission for).
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
BASELINE = REPO_ROOT / ".krit" / "oracle-fingerprints.json"
KRIT_BINARY = REPO_ROOT / "krit"

# (repo path relative to REPO_ROOT, rule-set label, extra krit flags)
CORPORA: list[tuple[str, str, list[str]]] = [
    ("playground/kotlin-webservice", "default", []),
    ("playground/kotlin-webservice", "all-rules", ["--all-rules"]),
    ("playground/android-app", "default", []),
    ("playground/android-app", "all-rules", ["--all-rules"]),
]


def _load_baseline() -> dict[tuple[str, str], dict[str, Any]]:
    if not BASELINE.exists():
        return {}
    raw = json.loads(BASELINE.read_text())
    out: dict[tuple[str, str], dict[str, Any]] = {}
    for entry in raw.get("entries", []):
        out[(entry["repo"], entry["ruleSet"])] = entry
    return out


def _run(repo: str, rule_set: str, flags: list[str]) -> dict[str, Any]:
    if not KRIT_BINARY.exists():
        print(
            f"error: {KRIT_BINARY} not found. Build with `go build -o krit ./cmd/krit/`.",
            file=sys.stderr,
        )
        sys.exit(2)
    cmd = [str(KRIT_BINARY), "--oracle-filter-fingerprint", *flags, repo]
    proc = subprocess.run(
        cmd,
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        print(
            f"error: {' '.join(cmd)} exited {proc.returncode}\nstderr:\n{proc.stderr}",
            file=sys.stderr,
        )
        sys.exit(2)
    data = json.loads(proc.stdout)
    data["repo"] = repo
    data["ruleSet"] = rule_set
    return data


def _entry_from(data: dict[str, Any]) -> dict[str, Any]:
    return {
        "repo": data["repo"],
        "ruleSet": data["ruleSet"],
        "fingerprint": data["fingerprint"],
        "markedFiles": data["markedFiles"],
        "totalFiles": data["totalFiles"],
        "allFiles": data["allFiles"],
    }


def _write_baseline(entries: list[dict[str, Any]]) -> None:
    payload = {
        "_comment": (
            "Oracle filter input-set fingerprints per (repo, rule-set). "
            "Regenerate via `python3 tools/oracle_fingerprint_check.py --update`. "
            "See issue #333."
        ),
        "entries": entries,
    }
    BASELINE.write_text(json.dumps(payload, indent=2) + "\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--update",
        action="store_true",
        help="Rewrite the baseline file with current fingerprints.",
    )
    args = parser.parse_args()

    observed = [_run(repo, rule_set, flags) for repo, rule_set, flags in CORPORA]

    if args.update:
        _write_baseline([_entry_from(d) for d in observed])
        print(f"Wrote {BASELINE.relative_to(REPO_ROOT)} with {len(observed)} entries.")
        return 0

    baseline = _load_baseline()
    drifted: list[tuple[dict[str, Any], dict[str, Any] | None]] = []
    for d in observed:
        key = (d["repo"], d["ruleSet"])
        entry = baseline.get(key)
        if entry is None or entry.get("fingerprint") != d["fingerprint"]:
            drifted.append((d, entry))

    if not drifted:
        print(f"Oracle fingerprint gate: OK ({len(observed)} corpora).")
        return 0

    print("Oracle filter fingerprint drift detected:\n", file=sys.stderr)
    for d, prev in drifted:
        old = prev.get("fingerprint") if prev else "<missing>"
        old_marked = prev.get("markedFiles") if prev else "-"
        old_total = prev.get("totalFiles") if prev else "-"
        print(
            f"  repo={d['repo']} ruleSet={d['ruleSet']}\n"
            f"    old: fingerprint={old} marked={old_marked}/{old_total}\n"
            f"    new: fingerprint={d['fingerprint']} marked={d['markedFiles']}/{d['totalFiles']} allFiles={d['allFiles']}",
            file=sys.stderr,
        )
    print(
        "\nIf this shift is intentional (you narrowed a rule's oracle filter "
        "or added/removed NeedsOracle on a rule), update the baseline:\n"
        "    python3 tools/oracle_fingerprint_check.py --update\n"
        "Then commit `.krit/oracle-fingerprints.json` with your change.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
