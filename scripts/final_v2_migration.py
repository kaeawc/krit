#!/usr/bin/env python3
"""Final batch: migrate remaining v1 Register(&...Rule{...}) calls to v2.Register(WrapAsV2(...)).

Handles only single-line Register(&Rule{...}) calls — the ones with multi-line
struct literals are left for manual review.
"""
import re
import sys
from pathlib import Path

RULES_DIR = Path(__file__).parent.parent / "internal" / "rules"

FILES = [
    "android_correctness.go",
    "android_correctness_checks.go",
    "android_icons.go",
    "android_source.go",
    "android_source_extra.go",
    "android_usability.go",
    "coroutines.go",
    "licensing.go",
]

# Single-line: `\tRegister(&FooRule{...})` (balanced braces within one line).
SINGLE_LINE_RE = re.compile(
    r"^(\t+)Register(Manifest|Resource|Gradle)?\((&\w+Rule\{.*\})\)$",
    re.MULTILINE,
)

# Multi-line: `\tRegister(&FooRule{AndroidRule: AndroidRule{\n\t\t...\n\t}})`
# Match the opening line, then any number of indented lines, then the closing `}}` line.
MULTI_LINE_RE = re.compile(
    r"^(\t+)Register(Manifest|Resource|Gradle)?\((&\w+Rule\{[^}]*\{)\n"
    r"((?:[ \t]+[^\n]*\n)*?)"  # inner lines
    r"[ \t]+\}\)\n",
    re.MULTILINE,
)


def migrate(content: str) -> tuple[str, int]:
    count = 0

    def single_repl(m: re.Match) -> str:
        nonlocal count
        count += 1
        indent, _, struct_literal = m.group(1), m.group(2), m.group(3)
        return f"{indent}v2.Register(WrapAsV2({struct_literal}))"

    content = SINGLE_LINE_RE.sub(single_repl, content)
    return content, count


def migrate_multiline(content: str) -> tuple[str, int]:
    """Handle multi-line struct literals spanning several lines."""
    count = 0
    lines = content.split("\n")
    out = []
    i = 0
    while i < len(lines):
        line = lines[i]
        m = re.match(
            r"^(\t+)Register(Manifest|Resource|Gradle)?\((&\w+Rule\{.*)$",
            line,
        )
        if m and not line.rstrip().endswith(")"):
            # Multi-line start — collect lines until matching `})` at same indent
            indent = m.group(1)
            struct_start = m.group(3)
            body_lines = [struct_start]
            depth = struct_start.count("{") - struct_start.count("}")
            i += 1
            while i < len(lines) and depth > 0:
                inner = lines[i]
                body_lines.append(inner)
                depth += inner.count("{") - inner.count("}")
                i += 1
            # Now the last line should end with `})` — strip the closing paren.
            last = body_lines[-1]
            if last.endswith("})"):
                body_lines[-1] = last[:-1]  # remove trailing `)`
                new_struct = "\n".join(body_lines)
                out.append(f"{indent}v2.Register(WrapAsV2({new_struct}))")
                count += 1
                continue
            # Fallback: leave untouched.
            out.extend([line] + body_lines[1:])
        else:
            out.append(line)
            i += 1
    return "\n".join(out), count


def ensure_import(content: str) -> str:
    """Make sure `v2 "github.com/kaeawc/krit/internal/rules/v2"` is imported."""
    if 'rules/v2"' in content:
        return content
    # Find first `import (` block.
    m = re.search(r'^import \(\n', content, re.MULTILINE)
    if not m:
        return content
    # Insert after.
    insert_at = m.end()
    return content[:insert_at] + '\tv2 "github.com/kaeawc/krit/internal/rules/v2"\n' + content[insert_at:]


def main() -> None:
    total = 0
    for filename in FILES:
        path = RULES_DIR / filename
        if not path.exists():
            print(f"SKIP {filename} (not found)")
            continue
        original = path.read_text()
        content = original
        # First handle single-line — easiest.
        content, c1 = migrate(content)
        # Then multi-line.
        content, c2 = migrate_multiline(content)
        if c1 + c2 > 0:
            content = ensure_import(content)
            path.write_text(content)
            print(f"{filename}: {c1} single-line + {c2} multi-line = {c1 + c2} migrated")
            total += c1 + c2
        else:
            print(f"{filename}: no changes")
    print(f"\nTotal: {total} migrations")


if __name__ == "__main__":
    main()
