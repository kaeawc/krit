#!/usr/bin/env python3
"""Exit 0 when every Krit action in a Bazel --execution_log_json_file was a cache hit.

    krit_actions_cached.py EXEC_LOG.json

The log is a stream of JSON objects, one per spawn. Same check as
scripts/bazel-e2e-local.sh.
"""

import json
import sys


def main(path):
    text = open(path, encoding="utf-8").read()
    decoder, pos, krit = json.JSONDecoder(), 0, []
    while pos < len(text):
        while pos < len(text) and text[pos].isspace():
            pos += 1
        if pos >= len(text):
            break
        entry, pos = decoder.raw_decode(text, pos)
        if entry.get("mnemonic") == "Krit":
            krit.append(entry)
    if not krit:
        print("no Krit actions in the execution log", file=sys.stderr)
        return 1
    missed = [e.get("targetLabel", "?") for e in krit if not (e.get("cacheHit") or e.get("remoteCacheHit"))]
    if missed:
        print("Krit actions re-ran instead of hitting the cache: " + ", ".join(missed), file=sys.stderr)
        return 1
    print(f"{len(krit)} Krit actions, all cache hits")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
