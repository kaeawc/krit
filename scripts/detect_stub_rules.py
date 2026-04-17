#!/usr/bin/env python3
"""Scan rule files for stub rules — rules with no real implementation.

A rule is flagged as a stub if EVERY one of its Check*/Collect*/Finalize
method bodies is trivial (`return nil` with no other code). A "pure
placeholder" is a rule that only has a `Check()` method and it's trivial.

Uses `go/ast`-style text parsing: finds method declarations, then uses
brace counting to extract the function body accurately.
"""
import re
import sys
from pathlib import Path
from collections import defaultdict

RULES = Path(__file__).parent.parent / "internal" / "rules"

SIGNATURE_RE = re.compile(
    r"^func\s+\(r\s+\*(\w+)\)\s+(Check\w*|Collect\w+|Finalize|NodeTypes|"
    r"AggregateNodeTypes|SetResolver|SetModuleIndex|Reset|AndroidDependencies)\b",
    re.MULTILINE,
)


def extract_body(content: str, signature_end_pos: int):
    """Starting at signature_end_pos (somewhere after `func ...`), find the
    opening `{`, then return the body (content between matched braces).
    """
    # Find the opening brace after the signature
    i = content.find("{", signature_end_pos)
    if i == -1:
        return None
    depth = 1
    j = i + 1
    in_string = None  # '"', '`', or None
    while j < len(content) and depth > 0:
        c = content[j]
        if in_string:
            if c == "\\" and in_string == '"':
                j += 2
                continue
            if c == in_string:
                in_string = None
        elif c in ('"', '`'):
            in_string = c
        elif c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return content[i + 1 : j]
        j += 1
    return None


def body_is_trivial(body: str) -> bool:
    """True if body strips to just `return nil` (or empty)."""
    # Remove line comments and block comments
    b = re.sub(r"//[^\n]*", "", body)
    b = re.sub(r"/\*.*?\*/", "", b, flags=re.DOTALL)
    b = re.sub(r"\s+", " ", b).strip()
    return b in ("return nil", "")


def scan_file(path: Path):
    content = path.read_text()
    methods = defaultdict(dict)  # rule_type -> {method: is_trivial}
    for m in SIGNATURE_RE.finditer(content):
        rule_type = m.group(1)
        method = m.group(2)
        body = extract_body(content, m.end())
        if body is None:
            continue
        methods[rule_type][method] = body_is_trivial(body)
    return methods


def main():
    all_methods = defaultdict(dict)
    file_of = {}
    for go_file in sorted(RULES.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        for rt, ms in scan_file(go_file).items():
            all_methods[rt].update(ms)
            file_of.setdefault(rt, go_file.name)

    skip_types = {
        "BaseRule", "FlatDispatchBase", "LineBase", "ManifestBase",
        "ResourceBase", "GradleBase", "LayoutResourceBase", "AndroidRule",
    }

    all_stub = []       # all Check* methods trivial (but has dispatch methods)
    pure_placeholder = []
    icon_style = []     # Check() trivial + AndroidDependencies declared

    for rt, ms in all_methods.items():
        if rt in skip_types:
            continue
        check_ms = {k: v for k, v in ms.items()
                    if k.startswith("Check") or k.startswith("Collect") or k == "Finalize"}
        if not check_ms:
            continue

        all_trivial = all(check_ms.values())
        only_check = set(check_ms.keys()) == {"Check"}
        has_android_deps = "AndroidDependencies" in ms

        if only_check and check_ms.get("Check"):
            if has_android_deps:
                icon_style.append((rt, file_of[rt]))
            else:
                pure_placeholder.append((rt, file_of[rt]))
        elif all_trivial and len(check_ms) > 1:
            all_stub.append((rt, file_of[rt], sorted(check_ms.keys())))
        elif all_trivial and "Check" not in check_ms:
            # Only a non-Check method and it's trivial (e.g., CheckLines → nil)
            all_stub.append((rt, file_of[rt], sorted(check_ms.keys())))

    total_rules = sum(1 for rt, ms in all_methods.items()
                      if rt not in skip_types
                      and any(k.startswith("Check") or k.startswith("Collect") or k == "Finalize"
                              for k in ms))

    print(f"Total rule structs scanned: {total_rules}")
    print()

    print("=== ALL-STUB RULES (every Check*/Collect/Finalize body is `return nil`) ===")
    if not all_stub:
        print("  (none)")
    for rt, fname, ms in sorted(all_stub):
        print(f"  {rt:<50s} ({fname})")
        print(f"    trivial methods: {', '.join(ms)}")
    print()

    print("=== PURE PLACEHOLDERS (only Check()→nil, no dispatch, no Android deps) ===")
    if not pure_placeholder:
        print("  (none)")
    for rt, fname in sorted(pure_placeholder):
        print(f"  {rt:<50s} ({fname})  [TRUE STUB — inflates rule count]")
    print()

    print("=== ICON-STYLE (Check()→nil + AndroidDependencies()) — routed via data pipeline ===")
    if not icon_style:
        print("  (none)")
    else:
        for rt, fname in sorted(icon_style):
            print(f"  {rt:<50s} ({fname})")
        print(f"  Total: {len(icon_style)} (intentional design — route via AndroidDep* metadata)")
    print()

    print("=== Summary ===")
    print(f"  True stubs (all-stub + pure placeholder): "
          f"{len(all_stub) + len(pure_placeholder)}")
    print(f"  Icon-style (intentional stubs): {len(icon_style)}")


if __name__ == "__main__":
    main()
