#!/usr/bin/env python3
"""Generate a machine-readable inventory of every krit rule.

Phase 1B of the CodegenRegistry roadmap. Emits build/rule_inventory.json
describing every rule's identity, default-active state, config options, fix
level, confidence, oracle filter, and concrete Go struct type. Used by the
downstream code generator that emits Meta() methods.

Post-Phase-3D structure. The legacy `applyRuleConfig` switch in config.go,
the literal DefaultInactive map in defaults.go, and knownRuleOptions() in
schema.go have all been deleted — the generated `zz_meta_*_gen.go` and the
hand-written sibling `meta_*.go` files are now the single source of truth
for rule descriptors. This script parses THOSE meta files for the option
set (Name, Aliases, Type, Default, Description, the Apply-target field),
the default-active flag, severity, and description.

Sources parsed (all Go files, read-only):
  1. internal/rules/*.go   — init() blocks for struct fields + registration
                             kind + adapter options (AdaptWith*).
  2. internal/rules/zz_meta_*_gen.go + meta_*.go — canonical Meta() bodies
                             (options, defaults, descriptor fields).
  3. internal/rules/*.go   — per-rule Confidence() / NodeTypes() /
                             OracleFilter() / FixLevel() method bodies.
"""
from __future__ import annotations

import datetime
import json
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parent.parent
RULES_DIR = ROOT / "internal" / "rules"
OUTPUT = ROOT / "build" / "rule_inventory.json"


# ---------- helpers ----------

def git_head_sha() -> str:
    try:
        out = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, stderr=subprocess.DEVNULL
        )
        return out.decode().strip()
    except Exception:
        return ""


def _decode_go_string_escapes(s: str) -> str:
    """Decode Go-style escape sequences (\\n \\t \\" \\\\ \\xHH \\uHHHH
    \\UHHHHHHHH) without double-decoding UTF-8 bytes. We walk the string
    character-by-character so non-ASCII literals pass through intact.
    """
    out: list[str] = []
    i = 0
    while i < len(s):
        c = s[i]
        if c != "\\":
            out.append(c)
            i += 1
            continue
        if i + 1 >= len(s):
            out.append(c)
            i += 1
            continue
        nxt = s[i + 1]
        simple = {
            "n": "\n", "t": "\t", "r": "\r", "b": "\b", "f": "\f",
            "0": "\0", "a": "\a", "v": "\v",
            '"': '"', "'": "'", "\\": "\\",
        }
        if nxt in simple:
            out.append(simple[nxt])
            i += 2
            continue
        if nxt == "x" and i + 3 < len(s):
            try:
                out.append(chr(int(s[i + 2:i + 4], 16)))
                i += 4
                continue
            except ValueError:
                pass
        if nxt == "u" and i + 5 < len(s):
            try:
                out.append(chr(int(s[i + 2:i + 6], 16)))
                i += 6
                continue
            except ValueError:
                pass
        if nxt == "U" and i + 9 < len(s):
            try:
                out.append(chr(int(s[i + 2:i + 10], 16)))
                i += 10
                continue
            except ValueError:
                pass
        # Unknown escape — pass through literally.
        out.append(c)
        i += 1
    return "".join(out)


def _parse_go_value(raw: str) -> Any:
    """Turn a Go literal fragment into a Python value (best-effort)."""
    s = raw.strip().rstrip(",").strip()
    if not s:
        return None
    if s in ("true", "false"):
        return s == "true"
    if s == "nil":
        return None
    if (s.startswith('"') and s.endswith('"')):
        body = s[1:-1]
        if "\\" not in body:
            # No escape sequences — return the raw string, preserving any
            # non-ASCII characters verbatim. (encode("utf-8").decode
            # ("unicode_escape") double-decodes UTF-8 bytes as Latin-1 and
            # mangles characters like → into mojibake.)
            return body
        try:
            # Go string literals share most escapes with Python: \n \t \r
            # \" \\ \xHH \uHHHH \UHHHHHHHH. Decoding as UTF-8 bytes via
            # unicode_escape handles those while leaving non-ASCII alone.
            # Round-trip only the ASCII portion through unicode_escape —
            # split the string so mojibake doesn't appear in multi-byte
            # sequences.
            return _decode_go_string_escapes(body)
        except Exception:
            return body
    if s.startswith("`") and s.endswith("`"):
        return s[1:-1]
    if re.fullmatch(r"-?\d+", s):
        return int(s)
    if re.fullmatch(r"-?\d+\.\d+", s):
        return float(s)
    m = re.fullmatch(r"\[\]string\s*\{(.*)\}", s, re.DOTALL)
    if m:
        inner = m.group(1).strip()
        if not inner:
            return []
        parts = re.findall(r'"((?:[^"\\]|\\.)*)"', inner)
        return [p.encode("utf-8").decode("unicode_escape", errors="replace") for p in parts]
    m = re.fullmatch(r"\[\]string\s*\(\s*nil\s*\)", s)
    if m:
        return []
    return {"__go_expr__": s}


# ---------- balanced scanning ----------

def _skip_rune_literal(src: str, i: int) -> int:
    """Advance past a Go rune literal starting at src[i] == "'".
    Returns the index just after the closing quote, or i+1 if malformed."""
    j = i + 1
    while j < len(src):
        if src[j] == "\\" and j + 1 < len(src):
            j += 2
            continue
        if src[j] == "'":
            return j + 1
        j += 1
    return i + 1


def _balance_parens(src: str, start: int) -> int:
    """Return index just after the matching close paren for the open at start."""
    depth = 0
    i = start
    in_str: str | None = None
    while i < len(src):
        c = src[i]
        if in_str:
            if c == "\\" and i + 1 < len(src):
                i += 2
                continue
            if c == in_str:
                in_str = None
            i += 1
            continue
        if c in ('"', '`'):
            in_str = c
            i += 1
            continue
        if c == "'":
            i = _skip_rune_literal(src, i)
            continue
        if c == "/" and i + 1 < len(src) and src[i + 1] == "/":
            nl = src.find("\n", i)
            i = len(src) if nl < 0 else nl
            continue
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1
    return -1


def _balance_braces(src: str, start: int) -> int:
    """Return index of matching close brace. `start` points at the open brace."""
    depth = 0
    i = start
    in_str: str | None = None
    while i < len(src):
        c = src[i]
        if in_str:
            if c == "\\" and i + 1 < len(src):
                i += 2
                continue
            if c == in_str:
                in_str = None
            i += 1
            continue
        if c in ('"', '`'):
            in_str = c
            i += 1
            continue
        if c == "'":
            i = _skip_rune_literal(src, i)
            continue
        if c == "/" and i + 1 < len(src) and src[i + 1] == "/":
            nl = src.find("\n", i)
            i = len(src) if nl < 0 else nl
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return i
        i += 1
    return -1


def _strip_go_line_comments(src: str) -> str:
    """Remove `//`-style line comments. Block comments are left intact (they
    don't appear in the Meta() bodies this script parses). String literals
    are preserved verbatim — a `//` inside `"..."` or `\`...\`` is kept.
    """
    out: list[str] = []
    i = 0
    in_str: str | None = None
    while i < len(src):
        c = src[i]
        if in_str:
            out.append(c)
            if c == "\\" and i + 1 < len(src):
                out.append(src[i + 1])
                i += 2
                continue
            if c == in_str:
                in_str = None
            i += 1
            continue
        if c in ('"', '`'):
            in_str = c
            out.append(c)
            i += 1
            continue
        if c == "/" and i + 1 < len(src) and src[i + 1] == "/":
            # Skip to end of line.
            nl = src.find("\n", i)
            if nl < 0:
                return "".join(out)
            i = nl
            continue
        out.append(c)
        i += 1
    return "".join(out)


def _split_toplevel_fields(body: str) -> list[str]:
    """Split a `{field:val, field:val}` body at top-level commas."""
    parts: list[str] = []
    depth_p = 0
    depth_b = 0
    depth_br = 0
    in_str: str | None = None
    buf: list[str] = []
    i = 0
    while i < len(body):
        c = body[i]
        if in_str:
            if c == "\\" and i + 1 < len(body):
                buf.append(c)
                buf.append(body[i + 1])
                i += 2
                continue
            if c == in_str:
                in_str = None
            buf.append(c)
            i += 1
            continue
        if c in ('"', '`'):
            in_str = c
            buf.append(c)
            i += 1
            continue
        if c == "(":
            depth_p += 1
        elif c == ")":
            depth_p -= 1
        elif c == "{":
            depth_b += 1
        elif c == "[":
            depth_br += 1
        elif c == "]":
            depth_br -= 1
        elif c == "}":
            depth_b -= 1
        elif c == "," and depth_p == 0 and depth_b == 0 and depth_br == 0:
            parts.append("".join(buf).strip())
            buf = []
            i += 1
            continue
        buf.append(c)
        i += 1
    tail = "".join(buf).strip()
    if tail:
        parts.append(tail)
    return parts


# ---------- struct literal parsing ----------

_STRUCT_LIT_RE = re.compile(r'&([A-Z][A-Za-z0-9_]*Rule)\s*\{')


def _parse_struct_literal(src: str, struct_start: int) -> tuple[str, dict, int]:
    """Parse &FooRule{...}. Returns (struct_name, fields, end_index)."""
    m = _STRUCT_LIT_RE.match(src, struct_start)
    if not m:
        return "", {}, -1
    struct_name = m.group(1)
    brace = src.find("{", m.end() - 1)
    end = _balance_braces(src, brace)
    if end < 0:
        return struct_name, {}, -1
    body = src[brace + 1:end]
    fields = _parse_fields(body)
    return struct_name, fields, end + 1


def _parse_fields(body: str) -> dict:
    """Parse a top-level field list into a dict.

    Nested BaseRule{...} and AndroidRule{...} structs are recursively parsed.
    BaseRule's RuleName/RuleSetName/Sev/Desc are promoted to `_BaseRule_`.
    Also handles positional fields and alcRule("Name", ...) helper calls that
    produce an AndroidRule — the first string arg becomes RuleName.
    """
    out: dict = {}
    parts = _split_toplevel_fields(body)
    for part_i, part in enumerate(parts):
        if not part:
            continue
        if ":" not in part:
            # Positional field — in &FooRule{alcRule("Name", ...)} shape,
            # the first positional field is typically an embedded AndroidRule
            # value produced by a helper call.
            value = part.strip()
            helper_m = re.match(r'([a-z][A-Za-z0-9_]*)\(', value)
            if helper_m and helper_m.group(1) in ("alcRule",):
                # alcRule("Name", "brief", ALSxxx, pri)
                args = _split_toplevel_fields(value[helper_m.end():].rstrip(")").strip())
                if args:
                    first = _parse_go_value(args[0])
                    if isinstance(first, str):
                        out["_BaseRule_"] = {
                            "RuleName": first,
                            "RuleSetName": {"__go_expr__": "androidRuleSet"},
                            "Sev": "",
                            "Desc": "",
                        }
            continue
        key, _, value = part.partition(":")
        key = key.strip()
        value = value.strip()
        # BaseRule: BaseRule{...} / AndroidRule: AndroidRule{...}
        nm = re.match(r'([A-Z][A-Za-z0-9_]*)\s*\{', value)
        if nm:
            inner_start = value.find("{")
            inner_end = _balance_braces(value, inner_start)
            if inner_end < 0:
                out[key] = {"__go_expr__": value}
                continue
            inner_body = value[inner_start + 1:inner_end]
            nested = _parse_fields(inner_body)
            type_name = nm.group(1)
            if type_name == "BaseRule":
                out["_BaseRule_"] = nested
            elif type_name in ("AndroidRule", "FlatDispatchBase", "GradleBase"):
                if "_BaseRule_" in nested:
                    out["_BaseRule_"] = nested["_BaseRule_"]
                out[key] = {"__nested_struct__": type_name, "fields": nested}
            else:
                out[key] = {"__nested_struct__": type_name, "fields": nested}
            continue
        # AndroidRule: alcRule("Name", ...)
        helper_m = re.match(r'([a-z][A-Za-z0-9_]*)\(', value)
        if helper_m and helper_m.group(1) in ("alcRule",):
            args = _split_toplevel_fields(value[helper_m.end():].rstrip(")").strip())
            if args:
                first = _parse_go_value(args[0])
                if isinstance(first, str):
                    out["_BaseRule_"] = {
                        "RuleName": first,
                        "RuleSetName": {"__go_expr__": "androidRuleSet"},
                        "Sev": "",
                        "Desc": "",
                    }
            out[key] = {"__go_expr__": value}
            continue
        out[key] = _parse_go_value(value)
    return out


# ---------- rule-set const resolution ----------

def _load_rule_set_consts(rules_files: list[Path]) -> dict[str, str]:
    """Find string consts from `const foo = "bar"`, `var foo = "bar"`, and
    `const (... foo = "bar" ...)` blocks."""
    consts: dict[str, str] = {}
    single_pat = re.compile(r'(?:const|var)\s+(\w+)\s*(?:[A-Za-z_]\w*\s*)?=\s*"([^"]+)"')
    block_pat = re.compile(r'^(?:const|var)\s*\(', re.MULTILINE)
    entry_pat = re.compile(
        r'^\s*([A-Za-z_]\w*)\s*(?:[A-Za-z_]\w*\s*)?=\s*"([^"]+)"',
        re.MULTILINE,
    )
    for f in rules_files:
        txt = f.read_text()
        for m in single_pat.finditer(txt):
            consts[m.group(1)] = m.group(2)
        for bm in block_pat.finditer(txt):
            start = txt.find("(", bm.start())
            end = _balance_parens(txt, start)
            if end < 0:
                continue
            body = txt[start + 1:end - 1]
            for em in entry_pat.finditer(body):
                consts[em.group(1)] = em.group(2)
    return consts


def _line_of_offset(text: str, off: int) -> int:
    return text.count("\n", 0, off) + 1


# ---------- init() registration scanning ----------

# Map from adapter suffix (after `v2.`) to the registration_kind label.
ADAPTER_KIND_MAP = {
    "AdaptFlatDispatch": "AdaptFlatDispatch",
    "AdaptLine": "AdaptLine",
    "AdaptCrossFile": "AdaptCrossFile",
    "AdaptParsedFiles": "AdaptParsedFiles",
    "AdaptAggregate": "AdaptAggregate",
    "AdaptModuleAware": "AdaptModuleAware",
    "AdaptManifest": "AdaptManifest",
    "AdaptResource": "AdaptResource",
    "AdaptGradle": "AdaptGradle",
}


def _extract_adapter_options(call_src: str) -> list[str]:
    """Find every v2.AdaptWithXxx(...) call inside an outer Register/adapt
    expression and return the full source-form string for each (including the
    `v2.` prefix). Preserves argument source verbatim so downstream codegen
    can re-emit it unchanged."""
    opts: list[str] = []
    # Match start of a v2.AdaptWith... call. We then balance parens to find
    # the full expression.
    pat = re.compile(r'v2\.AdaptWith[A-Z]\w*\s*\(')
    for m in pat.finditer(call_src):
        name_end = m.end() - 1  # points at '('
        call_close = _balance_parens(call_src, name_end)
        if call_close < 0:
            continue
        expr = call_src[m.start():call_close]
        # Normalize whitespace — collapse runs of whitespace/newline to a
        # single space, preserve string literals as-is.
        opts.append(_collapse_ws(expr))
    return opts


def _collapse_ws(expr: str) -> str:
    """Collapse all whitespace runs to single spaces, preserving string
    literal contents."""
    out: list[str] = []
    i = 0
    in_str: str | None = None
    prev_space = False
    while i < len(expr):
        c = expr[i]
        if in_str:
            out.append(c)
            if c == "\\" and i + 1 < len(expr):
                out.append(expr[i + 1])
                i += 2
                continue
            if c == in_str:
                in_str = None
            prev_space = False
            i += 1
            continue
        if c in ('"', '`'):
            in_str = c
            out.append(c)
            prev_space = False
            i += 1
            continue
        if c in (" ", "\t", "\n", "\r"):
            if not prev_space:
                out.append(" ")
                prev_space = True
            i += 1
            continue
        prev_space = False
        out.append(c)
        i += 1
    return "".join(out).strip()


def parse_init_registrations(rules_files: list[Path], consts: dict[str, str]) -> tuple[list[dict], list[str]]:
    """Scan every rule .go file for registration calls inside init() blocks."""
    regs: list[dict] = []
    warnings: list[str] = []
    reg_anchor_re = re.compile(
        r'(?:v2\.Register|RegisterManifest|RegisterResource|RegisterGradle)\s*\('
    )
    adapt_re = re.compile(
        r'v2\.(AdaptFlatDispatch|AdaptLine|AdaptCrossFile|AdaptParsedFiles|AdaptAggregate|AdaptModuleAware|AdaptManifest|AdaptResource|AdaptGradle)\s*\('
    )
    struct_decl_re = re.compile(
        r'(\w+)\s*:=\s*&([A-Z][A-Za-z0-9_]*Rule)\s*\{'
    )
    for gofile in rules_files:
        if gofile.name.endswith("_test.go"):
            continue
        text = gofile.read_text()
        rel = gofile.relative_to(ROOT).as_posix()
        # Scan both init() bodies and the generated registerAllRules()
        # body (post-Phase-3E, all rule registrations live there).
        init_regex = re.compile(
            r'^func\s+(?:init|registerAllRules)\s*\(\s*\)\s*\{', re.MULTILINE
        )
        for init_m in init_regex.finditer(text):
            body_start = init_m.end() - 1
            body_end = _balance_braces(text, body_start)
            if body_end < 0:
                continue
            body = text[body_start + 1:body_end]
            body_offset = body_start + 1

            # Find struct-decl helpers: `r := &FooRule{...}`.
            struct_decls = []
            for sm in struct_decl_re.finditer(body):
                brace_pos = sm.end() - 1
                end_pos = _balance_braces(body, brace_pos)
                if end_pos < 0:
                    continue
                fields = _parse_fields(body[brace_pos + 1:end_pos])
                struct_decls.append({
                    "var": sm.group(1),
                    "struct_name": sm.group(2),
                    "fields": fields,
                    "start": sm.start(),
                    "end": end_pos,
                })

            for reg in reg_anchor_re.finditer(body):
                anchor_start = reg.start()
                open_paren = reg.end() - 1
                close_paren = _balance_parens(body, open_paren)
                if close_paren < 0:
                    continue
                call_src = body[open_paren + 1:close_paren - 1]
                kind = reg.group(0).rstrip("( ").strip()
                reg_line = _line_of_offset(text, body_offset + anchor_start)

                entry: dict[str, Any] = {
                    "kind": kind,
                    "call_src": call_src.strip(),
                    "init_file": rel,
                    "init_line": reg_line,
                    "adapter_options": _extract_adapter_options(call_src),
                }

                # Case A: WrapAsV2(&FooRule{...}) inline.
                wrap_m = re.search(r'WrapAsV2\s*\(\s*', call_src)
                if wrap_m:
                    entry["registration_kind"] = "WrapAsV2"
                    amp_pos = call_src.find("&", wrap_m.end())
                    if amp_pos >= 0:
                        struct_name, fields, _ = _parse_struct_literal(call_src, amp_pos)
                        if struct_name:
                            entry["struct_type"] = struct_name
                            entry["struct_fields"] = fields
                            _extract_base_rule(entry, fields, consts)
                            regs.append(entry)
                            continue
                    warnings.append(f"{rel}:{reg_line}: WrapAsV2 without parseable struct literal")
                    continue

                # Case B: v2.AdaptXxx(r.RuleName, ...) referencing a
                # previously-declared var `r`. The adapt call is usually
                # nested inside the outer Register(...).
                adapt_m = adapt_re.search(call_src)
                if adapt_m:
                    adapter_name = adapt_m.group(1)
                    # RegisterManifest/RegisterResource/RegisterGradle wrap the
                    # adapter to produce the matching registration_kind value.
                    reg_kind = ADAPTER_KIND_MAP.get(adapter_name, adapter_name)
                    entry["registration_kind"] = reg_kind
                    entry["adapter"] = adapter_name
                    cm = re.search(r'AdaptWithConfidence\(\s*([0-9.]+)\s*\)', call_src)
                    if cm:
                        entry["confidence"] = float(cm.group(1))
                    fm = re.search(r'AdaptWithFix\(\s*([A-Za-z_0-9.]+)\s*\)', call_src)
                    if fm:
                        entry["fix_level"] = fm.group(1)
                    needs = [nm.group(1).strip() for nm in re.finditer(
                        r'AdaptWithNeeds\(\s*([^)]+)\s*\)', call_src)]
                    if needs:
                        entry["needs_exprs"] = needs
                    if "AdaptWithOracle(" in call_src:
                        entry["has_oracle"] = True

                    # Find the first positional argument of the adapt call by
                    # balancing parens after `v2.AdaptXxx(`.
                    adapt_open = call_src.find("(", adapt_m.end() - 1)
                    adapt_close = _balance_parens(call_src, adapt_open)
                    if adapt_open >= 0 and adapt_close > 0:
                        adapt_args = call_src[adapt_open + 1:adapt_close - 1]
                        first_arg_m = re.match(
                            r'\s*([A-Za-z_]\w*)\.RuleName\b', adapt_args
                        )
                        if first_arg_m:
                            var_ref = first_arg_m.group(1)
                            best = None
                            for sd in struct_decls:
                                if sd["var"] == var_ref and sd["end"] < anchor_start:
                                    best = sd
                            if best:
                                entry["struct_type"] = best["struct_name"]
                                entry["struct_fields"] = best["fields"]
                                _extract_base_rule(entry, best["fields"], consts)
                                regs.append(entry)
                                continue
                    warnings.append(
                        f"{rel}:{reg_line}: {adapter_name} call unresolved"
                    )
                    regs.append(entry)
                    continue

                # Case C: RegisterManifest(&FooRule{...}) / RegisterResource / RegisterGradle.
                amp_pos = call_src.find("&")
                if kind in ("RegisterManifest", "RegisterResource", "RegisterGradle") and amp_pos >= 0:
                    struct_name, fields, _ = _parse_struct_literal(call_src, amp_pos)
                    if struct_name:
                        # Bare RegisterXxx(&FooRule{...}) — treat as Adapt<Kind>
                        # so Phase 3E can re-emit the v1-style registration
                        # verbatim.
                        reg_kind = {
                            "RegisterManifest": "AdaptManifest",
                            "RegisterResource": "AdaptResource",
                            "RegisterGradle": "AdaptGradle",
                        }[kind]
                        entry["registration_kind"] = reg_kind
                        entry["struct_type"] = struct_name
                        entry["struct_fields"] = fields
                        _extract_base_rule(entry, fields, consts)
                        regs.append(entry)
                        continue

                # Case C2: v2.Register(&v2.Rule{..., OriginalV1: r, ...}) —
                # post-FindingRepresentationUnification shape. The actual
                # rule struct was declared earlier as `r := &FooRule{...}`
                # and the v2.Rule wrapper references it via OriginalV1.
                if kind == "v2.Register":
                    rule_struct_m = re.match(
                        r'\s*&v2\.Rule\s*\{', call_src
                    )
                    if rule_struct_m:
                        # Find OriginalV1: <varname> inside the v2.Rule literal.
                        orig_m = re.search(
                            r'OriginalV1\s*:\s*([A-Za-z_]\w*)', call_src
                        )
                        if orig_m:
                            var_ref = orig_m.group(1)
                            best = None
                            for sd in struct_decls:
                                if sd["var"] == var_ref and sd["end"] < anchor_start:
                                    best = sd
                            if best:
                                entry["registration_kind"] = "WrapAsV2"
                                entry["struct_type"] = best["struct_name"]
                                entry["struct_fields"] = best["fields"]
                                _extract_base_rule(entry, best["fields"], consts)
                                regs.append(entry)
                                continue

                # Case D: v2.Register(existingVar) without WrapAsV2.
                id_m = re.match(r'\s*([A-Za-z_]\w*)\s*$', call_src)
                if id_m:
                    var_ref = id_m.group(1)
                    best = None
                    for sd in struct_decls:
                        if sd["var"] == var_ref and sd["end"] < anchor_start:
                            best = sd
                    if best:
                        entry["registration_kind"] = "unknown"
                        entry["struct_type"] = best["struct_name"]
                        entry["struct_fields"] = best["fields"]
                        _extract_base_rule(entry, best["fields"], consts)
                        regs.append(entry)
                        continue

                entry["registration_kind"] = "unknown"
                warnings.append(f"{rel}:{reg_line}: unresolved registration kind={kind}")
                regs.append(entry)
    return regs, warnings


def _extract_base_rule(entry: dict, fields: dict, consts: dict[str, str]) -> None:
    """Promote RuleName / RuleSetName / Sev / Desc from BaseRule into entry."""
    base = fields.get("_BaseRule_")
    if not base:
        # Some fields may embed BaseRule via AndroidRule — try nested fields.
        for v in fields.values():
            if isinstance(v, dict) and "fields" in v:
                if "_BaseRule_" in v["fields"]:
                    base = v["fields"]["_BaseRule_"]
                    break
    if not base:
        return
    entry["id"] = base.get("RuleName")
    rsn = base.get("RuleSetName")
    if isinstance(rsn, dict) and "__go_expr__" in rsn:
        rsn = consts.get(rsn["__go_expr__"], rsn["__go_expr__"])
    entry["ruleset"] = rsn
    entry["severity"] = base.get("Sev", "")
    entry["description"] = base.get("Desc", "") or ""


# ---------- Meta() parsing ----------

# OptType constant -> inventory go_type string.
_OPT_TYPE_MAP = {
    "OptInt": "int",
    "OptBool": "bool",
    "OptString": "string",
    "OptStringList": "[]string",
    "OptRegex": "regex",
}


def _split_meta_funcs(text: str) -> list[tuple[str, str]]:
    """Yield (struct_type, body) for each `func (r *FooRule) Meta() ... {...}`
    in the file. body is the contents of the outer {}."""
    out: list[tuple[str, str]] = []
    pat = re.compile(
        r'^func\s*\(\s*\w+\s+\*([A-Z][A-Za-z0-9_]*Rule)\s*\)\s*Meta\s*\(\s*\)\s*registry\.RuleDescriptor\s*\{',
        re.MULTILINE,
    )
    for m in pat.finditer(text):
        brace = m.end() - 1
        end = _balance_braces(text, brace)
        if end < 0:
            continue
        body = text[brace + 1:end]
        out.append((m.group(1), body))
    return out


def _parse_meta_body(body: str) -> dict:
    """Parse the body of a Meta() function, returning a dict of descriptor
    fields + parsed options list."""
    # Strip Go line comments — hand-written meta_*.go files may interleave
    # documentation comments between ConfigOption fields, and those would
    # otherwise confuse the comma-split parser.
    body = _strip_go_line_comments(body)
    # Find the `return registry.RuleDescriptor{...}` block.
    ret = re.search(r'return\s+registry\.RuleDescriptor\s*\{', body)
    if not ret:
        return {}
    brace = ret.end() - 1
    end = _balance_braces(body, brace)
    if end < 0:
        return {}
    inner = body[brace + 1:end]
    # Parse at top-level: each entry is `Name: value,`.
    fields_raw = _split_toplevel_fields(inner)
    out: dict[str, Any] = {}
    options_body: str | None = None
    for part in fields_raw:
        if not part:
            continue
        if ":" not in part:
            continue
        key, _, value = part.partition(":")
        key = key.strip()
        value = value.strip()
        if key == "Options":
            # value is `[]registry.ConfigOption{ {...}, {...}, }`
            options_body = value
            continue
        if key in ("ID", "RuleSet", "Severity", "Description", "SourceHash"):
            out[key] = _parse_go_value(value)
        elif key == "DefaultActive":
            out[key] = value.strip().rstrip(",") == "true"
        elif key == "FixLevel":
            out[key] = _parse_go_value(value)
        elif key == "Confidence":
            try:
                out[key] = float(value.strip().rstrip(","))
            except ValueError:
                out[key] = None
        elif key == "Oracle":
            oracle_val = value.strip().rstrip(",")
            if oracle_val == "nil":
                out[key] = None
            else:
                # e.g. "&registry.OracleFilter{}" or "&registry.OracleFilter{AllFiles: true}"
                out[key] = {"__go_expr__": oracle_val}

    out["Options"] = _parse_meta_options(options_body) if options_body else []
    return out


def _parse_meta_options(raw: str) -> list[dict]:
    """Parse the Options list from a Meta() body. raw is e.g.
    `[]registry.ConfigOption{ {Name: "...", Type: registry.OptInt, ...}, }`.
    Returns a list of dicts with keys: Name, Aliases, Type, Default,
    Description, Field (the struct field name extracted from Apply)."""
    # Locate the outer '[]registry.ConfigOption{...}' braces.
    lb = raw.find("{")
    if lb < 0:
        return []
    end = _balance_braces(raw, lb)
    if end < 0:
        return []
    body = raw[lb + 1:end]
    opts: list[dict] = []
    # Iterate depth 0 -> 1 transitions to find each `{ ... }` entry.
    i = 0
    depth = 0
    start_i = -1
    in_str: str | None = None
    while i < len(body):
        c = body[i]
        if in_str:
            if c == "\\" and i + 1 < len(body):
                i += 2
                continue
            if c == in_str:
                in_str = None
            i += 1
            continue
        if c in ('"', '`'):
            in_str = c
            i += 1
            continue
        if c == "{":
            if depth == 0:
                start_i = i
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0 and start_i >= 0:
                inner = body[start_i + 1:i]
                opt = _parse_meta_option_entry(inner)
                if opt:
                    opts.append(opt)
                start_i = -1
        i += 1
    return opts


def _parse_meta_option_entry(raw: str) -> dict:
    """Parse a single ConfigOption literal body into a dict."""
    # Find Apply closure first — it may contain `}` braces that confuse a
    # naive splitter. _split_toplevel_fields handles this correctly because
    # depth tracking includes `{}`.
    out: dict[str, Any] = {
        "Name": "",
        "Aliases": [],
        "Type": "",
        "Default": None,
        "Description": "",
        "Field": "",
    }
    parts = _split_toplevel_fields(raw)
    for part in parts:
        if not part or ":" not in part:
            continue
        key, _, value = part.partition(":")
        key = key.strip()
        value = value.strip().rstrip(",").strip()
        if key == "Name":
            out["Name"] = _parse_go_value(value)
        elif key == "Aliases":
            # e.g. []string{"threshold"}
            parsed = _parse_go_value(value)
            if isinstance(parsed, list):
                out["Aliases"] = parsed
        elif key == "Type":
            # registry.OptInt / registry.OptBool / registry.OptString / etc.
            m = re.match(r'registry\.(Opt\w+)', value)
            if m:
                out["Type"] = _OPT_TYPE_MAP.get(m.group(1), m.group(1).lower())
        elif key == "Default":
            out["Default"] = _parse_meta_default(out["Type"], value)
        elif key == "Description":
            out["Description"] = _parse_go_value(value) or ""
        elif key == "Apply":
            out["Field"] = _extract_apply_field(value)
    return out


def _parse_meta_default(go_type: str, value: str) -> Any:
    """Turn a Go default literal into an inventory Python value."""
    v = value.strip().rstrip(",").strip()
    if v == "nil" or v == "[]string(nil)":
        # Inventory historically used "" for empty-string defaults; keep
        # the caller's type hint and default to zero-value.
        if go_type == "string":
            return ""
        if go_type == "[]string":
            return []
        return None
    return _parse_go_value(v)


def _extract_apply_field(apply_src: str) -> str:
    """Given the source of an Apply closure, extract the struct field name.

    The generated closure shape is:
        func(target interface{}, value interface{}) {
            target.(*StructName).FieldName = value.(...)
        }
    Hand-written closures may be multi-line or use different value
    expressions; we only need the FieldName.
    """
    # Collapse newlines/whitespace so multi-line closures (hand-written metas
    # may split the assignment RHS across lines) still match.
    compact = re.sub(r'\s+', ' ', apply_src)
    # Direct-form: target.(*FooRule).FieldName = value.(...)
    m = re.search(r'target\.\(\*[A-Za-z_]\w*\)\.([A-Za-z_]\w*)\s*=', compact)
    if m:
        return m.group(1)
    # Aliased-form: `rule := target.(*FooRule); rule.FieldName = value`.
    # Hand-written metas sometimes bind a local for multi-field writes —
    # we report the FIRST bound field as the "primary" since the inventory
    # schema assumes one field per option.
    alias_m = re.search(r'([A-Za-z_]\w*)\s*:=\s*target\.\(\*[A-Za-z_]\w*\)', compact)
    if alias_m:
        alias = alias_m.group(1)
        f = re.search(rf'\b{re.escape(alias)}\.([A-Za-z_]\w*)\s*=', compact)
        if f:
            return f.group(1)
    return ""


def parse_meta_files(rules_files: list[Path]) -> dict[str, dict]:
    """Parse every Meta() body in the rules package. Returns a map keyed by
    struct_type -> descriptor dict.

    Looks at both:
      - generated `zz_meta_*_gen.go`
      - hand-written `meta_*.go` siblings (excluded from codegen)
    """
    out: dict[str, dict] = {}
    for f in rules_files:
        name = f.name
        if not (name.startswith("zz_meta_") or name.startswith("meta_")):
            continue
        if name.endswith("_test.go"):
            continue
        text = f.read_text()
        for struct_name, body in _split_meta_funcs(text):
            parsed = _parse_meta_body(body)
            if parsed:
                out[struct_name] = parsed
    return out


# ---------- method-body scraping ----------

_CONFIDENCE_RE = re.compile(
    r'func\s+\(\s*\w+\s+\*([A-Z][A-Za-z0-9_]*Rule)\s*\)\s*Confidence\s*\(\s*\)\s*float64\s*\{\s*return\s+([0-9.]+)'
)
_NODETYPES_RE = re.compile(
    r'func\s+\(\s*\w+\s+\*([A-Z][A-Za-z0-9_]*Rule)\s*\)\s*NodeTypes\s*\(\s*\)\s*\[\]string\s*\{\s*return\s+\[\]string\s*\{([^}]*)\}',
    re.DOTALL,
)
_ORACLE_FILTER_RE = re.compile(
    r'func\s+\(\s*\w+\s+\*([A-Z][A-Za-z0-9_]*Rule)\s*\)\s*OracleFilter\s*\(\s*\)\s*\*OracleFilter\s*\{\s*return\s+([A-Za-z_][A-Za-z0-9_]*)',
)
_FIXLEVEL_RE = re.compile(
    r'func\s+\(\s*\w+\s+\*([A-Z][A-Za-z0-9_]*Rule)\s*\)\s*FixLevel\s*\(\s*\)\s*FixLevel\s*\{\s*return\s+(FixCosmetic|FixIdiomatic|FixSemantic)',
)
_FIXLEVEL_MAP = {
    "FixCosmetic": "cosmetic",
    "FixIdiomatic": "idiomatic",
    "FixSemantic": "semantic",
}


def scrape_method_bodies(rules_files: list[Path]) -> dict[str, dict]:
    out: dict[str, dict] = defaultdict(dict)
    for f in rules_files:
        if f.name.endswith("_test.go"):
            continue
        text = f.read_text()
        for m in _CONFIDENCE_RE.finditer(text):
            out[m.group(1)]["confidence"] = float(m.group(2))
        for m in _NODETYPES_RE.finditer(text):
            body = m.group(2)
            types = re.findall(r'"([^"]+)"', body)
            if types:
                out[m.group(1)]["node_types"] = types
        for m in _ORACLE_FILTER_RE.finditer(text):
            out[m.group(1)]["oracle"] = m.group(2)
        for m in _FIXLEVEL_RE.finditer(text):
            out[m.group(1)]["fix_level"] = _FIXLEVEL_MAP[m.group(2)]
    return out


# ---------- struct location ----------

def _locate_struct_files(rules_files: list[Path]) -> dict[str, str]:
    out: dict[str, str] = {}
    pat = re.compile(r'^type\s+([A-Z][A-Za-z0-9_]*Rule)\s+struct', re.MULTILINE)
    for f in rules_files:
        if f.name.endswith("_test.go"):
            continue
        text = f.read_text()
        for m in pat.finditer(text):
            name = m.group(1)
            out.setdefault(name, f.relative_to(ROOT).as_posix())
    return out


# ---------- merge ----------

def build_inventory() -> dict:
    warnings: list[str] = []
    rules_files = sorted(RULES_DIR.glob("*.go"))
    consts = _load_rule_set_consts(rules_files)

    regs, reg_warnings = parse_init_registrations(rules_files, consts)
    warnings.extend(reg_warnings)
    meta_by_struct = parse_meta_files(rules_files)
    method_info = scrape_method_bodies(rules_files)
    struct_files = _locate_struct_files(rules_files)

    # Dedupe by (struct_type, id). Prefer entries with struct_fields.
    seen: dict[tuple, dict] = {}
    for r in regs:
        sid = r.get("id")
        st = r.get("struct_type")
        if not sid or not st:
            continue
        key = (st, sid)
        prev = seen.get(key)
        if prev is None or (not prev.get("struct_fields") and r.get("struct_fields")):
            seen[key] = r

    out_rules: list[dict] = []
    for (struct_type, rule_id), reg in sorted(seen.items(), key=lambda x: x[0][1]):
        rule_warnings: list[str] = []
        meta = meta_by_struct.get(struct_type, {})

        # Prefer Meta() for descriptor fields; fall back to init-parsed
        # values (useful if Meta() is missing or skipped).
        # Prefer init() BaseRule values for ruleset/severity/description
        # when they are non-empty — the source .go file is the canonical
        # author-entered copy, and re-parsing it avoids any mojibake that
        # may have leaked into zz_meta_*_gen.go on a previous bad-run.
        # Fall back to Meta() for the (rare) case where BaseRule is empty
        # but Meta() still carries useful data.
        ruleset = reg.get("ruleset", "") or meta.get("RuleSet") or ""
        if isinstance(ruleset, dict):
            ruleset = ruleset.get("__go_expr__", "")
        severity = reg.get("severity", "") or (meta.get("Severity") or "")
        description = reg.get("description", "") or (meta.get("Description") or "")
        struct_fields = reg.get("struct_fields") or {}

        if "DefaultActive" in meta:
            default_active = bool(meta["DefaultActive"])
        else:
            # No Meta() — assume active (matches codegen default).
            default_active = True
            if struct_type not in meta_by_struct:
                rule_warnings.append(
                    f"{struct_type}: no Meta() descriptor found; defaulting to active"
                )

        # Fix level: method body wins, then Meta(), then adapter hint.
        fix_level = method_info.get(struct_type, {}).get("fix_level")
        if not fix_level:
            mfl = meta.get("FixLevel") if isinstance(meta.get("FixLevel"), str) else None
            if mfl:
                fix_level = mfl
        if not fix_level:
            adapter_fix = reg.get("fix_level")
            if adapter_fix:
                name = adapter_fix.rsplit(".", 1)[-1]
                fix_level = _FIXLEVEL_MAP.get(name, name.lower())
        fix_level = fix_level or ""

        confidence = method_info.get(struct_type, {}).get("confidence")
        if confidence is None:
            confidence = meta.get("Confidence")
        if confidence is None:
            confidence = reg.get("confidence")

        node_types = method_info.get(struct_type, {}).get("node_types") or []
        oracle = method_info.get(struct_type, {}).get("oracle")
        if not oracle:
            moracle = meta.get("Oracle")
            if isinstance(moracle, dict) and "__go_expr__" in moracle:
                # Canonical filter name can't be recovered from the literal;
                # mark as "unknown" for downstream visibility.
                oracle = "unknown"
        if not oracle and reg.get("has_oracle"):
            oracle = "unknown"

        # Initializer fields: only concrete rule's own fields. Skip any
        # nested-struct expressions (AndroidRule{...}, FlatDispatchBase{...}, etc.)
        struct_defaults: dict[str, Any] = {}
        for k, v in struct_fields.items():
            if k == "_BaseRule_":
                continue
            if isinstance(v, dict) and "__nested_struct__" in v:
                continue
            if isinstance(v, dict) and "__go_expr__" in v:
                continue
            struct_defaults[k] = v

        # Options now come from Meta(). Preserve the historical inventory
        # schema: field, go_type, yaml_key, aliases, default, description.
        options: list[dict] = []
        for mo in meta.get("Options", []):
            yaml_key = mo.get("Name") or ""
            go_type = mo.get("Type") or ""
            field = mo.get("Field") or ""
            aliases = list(mo.get("Aliases") or [])
            default_val = mo.get("Default")
            # Prefer struct literal default when present (matches historical
            # inventory behavior: use the struct-init value over the Meta
            # literal, because struct defaults track runtime state).
            if field and field in struct_defaults:
                default_val = struct_defaults[field]
            options.append({
                "field": field,
                "go_type": go_type,
                "yaml_key": yaml_key,
                "aliases": aliases,
                "default": default_val,
                "description": mo.get("Description") or "",
            })

        out_rules.append({
            "struct_type": struct_type,
            "file": struct_files.get(struct_type, ""),
            "init_file": reg.get("init_file", ""),
            "init_line": reg.get("init_line", 0),
            "id": rule_id,
            "ruleset": ruleset,
            "severity": severity,
            "description": description,
            "default_active": default_active,
            "fix_level": fix_level,
            "confidence": confidence,
            "node_types": node_types,
            "oracle": oracle,
            "needs": reg.get("needs_exprs", []),
            "struct_defaults": struct_defaults,
            "options": options,
            "registration_kind": reg.get("registration_kind", "unknown"),
            "adapter_options": reg.get("adapter_options", []),
            "warnings": rule_warnings,
        })

    # Every Meta() struct should have a matching registered rule (either
    # through the registry or through a hand-written adapter).
    structs_found = {r["struct_type"] for r in out_rules}
    for struct in sorted(meta_by_struct.keys()):
        if struct not in structs_found:
            warnings.append(f"Meta() for {struct}: no matching registered rule")

    # Stats.
    rules_by_ruleset: dict[str, int] = defaultdict(int)
    rules_with_config = 0
    rules_default_inactive = 0
    reg_kind_counts: dict[str, int] = defaultdict(int)
    for r in out_rules:
        rs = r.get("ruleset") or "unknown"
        rules_by_ruleset[rs] += 1
        if r["options"]:
            rules_with_config += 1
        if not r["default_active"]:
            rules_default_inactive += 1
        reg_kind_counts[r["registration_kind"]] += 1

    return {
        "generated_at": datetime.datetime.utcnow().replace(microsecond=0).isoformat() + "Z",
        "source_commit": git_head_sha(),
        "rules": out_rules,
        "warnings": warnings,
        "stats": {
            "total_rules": len(out_rules),
            "rules_with_config": rules_with_config,
            "rules_default_inactive": rules_default_inactive,
            "rules_by_ruleset": dict(sorted(rules_by_ruleset.items())),
            "registration_kinds": dict(sorted(reg_kind_counts.items())),
        },
    }


def main() -> None:
    inv = build_inventory()
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with OUTPUT.open("w") as fh:
        json.dump(inv, fh, indent=2, default=str)
        fh.write("\n")
    stats = inv["stats"]
    rule_warnings = sum(1 for r in inv["rules"] if r["warnings"])
    print(f"Wrote {OUTPUT.relative_to(ROOT)}", file=sys.stderr)
    print(f"  total_rules           = {stats['total_rules']}", file=sys.stderr)
    print(f"  rules_with_config     = {stats['rules_with_config']}", file=sys.stderr)
    print(f"  rules_default_inactive= {stats['rules_default_inactive']}", file=sys.stderr)
    print(f"  top-level warnings    = {len(inv['warnings'])}", file=sys.stderr)
    print(f"  rules w/ warnings     = {rule_warnings}", file=sys.stderr)
    print(f"  registration_kinds    = {stats['registration_kinds']}", file=sys.stderr)


if __name__ == "__main__":
    main()
