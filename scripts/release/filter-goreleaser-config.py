#!/usr/bin/env python3
"""Filter .goreleaser.yml down to a per-OS subset of builds and archives.

`goreleaser release` (the OSS variant) does not accept --id flags, so the
matrix-funnel workflow can't filter at the CLI. Instead each matrix
runner generates a custom config containing only the build ids it owns
and only archives that reference at least one of those ids.

Usage:
    python3 filter-goreleaser-config.py <ids-comma-separated>

The filtered YAML is written to stdout.
"""

from __future__ import annotations

import sys
from pathlib import Path

import yaml


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print("usage: filter-goreleaser-config.py <id1,id2,...>", file=sys.stderr)
        return 2

    keep = {x.strip() for x in argv[1].split(",") if x.strip()}
    if not keep:
        print("error: no ids provided", file=sys.stderr)
        return 2

    src = Path(".goreleaser.yml")
    data = yaml.safe_load(src.read_text())

    builds = data.get("builds", [])
    data["builds"] = [b for b in builds if b.get("id") in keep]
    if not data["builds"]:
        print(f"error: no builds matched ids {sorted(keep)}", file=sys.stderr)
        return 1

    new_archives = []
    for a in data.get("archives", []):
        a_ids = [i for i in a.get("ids", []) if i in keep]
        if a_ids:
            a["ids"] = a_ids
            new_archives.append(a)
    data["archives"] = new_archives

    yaml.safe_dump(data, sys.stdout, sort_keys=False)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
