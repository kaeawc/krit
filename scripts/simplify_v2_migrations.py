#!/usr/bin/env python3
"""Simplify v2 migrations by replacing manual adapter calls with WrapAsV2.

Transforms:
    {
        r := &RuleType{...}
        v2.Register(v2.AdaptFlatDispatch(r.RuleName, r.RuleSetName, r.Description(), v2.Severity(r.Sev), r.NodeTypes(), r.CheckFlatNode, ...opts))
    }

Into:
    v2.Register(WrapAsV2(&RuleType{...}))

This works because WrapAsV2 auto-detects ConfidenceProvider, FixLevelRule,
OracleFilterProvider, and TypeAwareRule interfaces from the struct instance.
"""
import re
from pathlib import Path

RULES_DIR = Path(__file__).parent.parent / "internal" / "rules"

# Match a scoped block:
#   \t{
#   \t\tr := &RuleType{...literal...}
#   \t\tv2.Register(v2.AdaptFlatDispatch(...))
#   \t}
# or the same pattern for AdaptLine.
BLOCK_RE = re.compile(
    r"([ \t]*)\{\n"
    r"([ \t]+)r := (&\w+\{[^}]*\}(?:\}[^{}]*\})*)\n"
    r"[ \t]+v2\.Register\(v2\.Adapt(?:FlatDispatch|Line)\([^)]*(?:\([^)]*\)[^)]*)*\)\)\n"
    r"\1\}",
    re.MULTILINE,
)

# Match deeper nesting for struct literals like AndroidRule: AndroidRule{BaseRule: BaseRule{...}, ...}
COMPLEX_BLOCK_RE = re.compile(
    r"([ \t]*)\{\n"
    r"([ \t]+)r := (&\w+\{(?:[^{}]|\{[^{}]*\})*\})\n"
    r"[ \t]+v2\.Register\(v2\.Adapt(?:FlatDispatch|Line)\([^)]*(?:\([^)]*\)[^)]*)*\)\)\n"
    r"\1\}",
    re.MULTILINE,
)


def transform(content: str, file_path: Path) -> tuple[str, int]:
    """Transform v2 migrations in-place. Returns (new_content, count)."""
    count = 0

    def repl(m: re.Match) -> str:
        nonlocal count
        count += 1
        outer_indent = m.group(1)
        struct_literal = m.group(3)
        return f"{outer_indent}v2.Register(WrapAsV2({struct_literal}))"

    new_content = COMPLEX_BLOCK_RE.sub(repl, content)
    return new_content, count


def main() -> None:
    total = 0
    files_changed = 0
    for go_file in sorted(RULES_DIR.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        content = go_file.read_text()
        new_content, count = transform(content, go_file)
        if count > 0:
            go_file.write_text(new_content)
            print(f"  {go_file.name}: {count} migrations simplified")
            files_changed += 1
            total += count
    print(f"\nTotal: {total} migrations simplified across {files_changed} files")


if __name__ == "__main__":
    main()
