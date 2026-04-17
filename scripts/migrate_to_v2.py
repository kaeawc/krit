#!/usr/bin/env python3
"""Diagnostic tool for migrating rule init() registrations from v1 to v2.

Usage:
    python3 scripts/migrate_to_v2.py                    # diagnostic report
    python3 scripts/migrate_to_v2.py --generate FILE.go # generate v2 code for a specific file
"""

import argparse
import os
import re
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path

RULES_DIR = os.path.join(os.path.dirname(__file__), "..", "internal", "rules")

# Base type -> interface category
BASE_TYPE_MAP = {
    "FlatDispatchBase": "FlatDispatchRule",
    "LineBase": "LineRule",
    "ManifestBase": "ManifestRule",
    "ResourceBase": "ResourceRule",
    "LayoutResourceBase": "LayoutResourceRule",
    "GradleBase": "GradleRule",
    "IconBase": "IconRule",
    "AndroidRule": "LegacyAndroid",
}

# Fields that are part of the standard base — not "extra config"
STANDARD_FIELDS = {"BaseRule", "FlatDispatchBase", "LineBase", "ManifestBase",
                   "ResourceBase", "LayoutResourceBase", "GradleBase", "IconBase",
                   "AndroidRule"}


@dataclass
class RuleInfo:
    name: str
    struct_type: str
    base_type: str  # e.g. "FlatDispatchBase", "LineBase"
    interface: str  # e.g. "FlatDispatchRule", "LineRule"
    file: str
    extra_fields: list = field(default_factory=list)
    has_confidence: bool = False
    has_fix_level: bool = False
    has_oracle_filter: bool = False
    has_set_resolver: bool = False
    has_check_flat_node: bool = False
    has_check_lines: bool = False
    has_node_types: bool = False
    register_line: str = ""
    rule_set: str = ""
    severity: str = ""
    description: str = ""


def parse_struct_types(content: str, filepath: str) -> dict:
    """Parse struct type definitions and their embedded types / extra fields."""
    structs = {}
    # Match: type FooRule struct { ... }
    # Handle both single-line and multi-line struct defs
    pattern = re.compile(
        r"type\s+(\w+Rule\w*)\s+struct\s*\{([^}]*)\}",
        re.DOTALL,
    )
    for m in pattern.finditer(content):
        struct_name = m.group(1)
        body = m.group(2)
        # Split on newlines AND semicolons (Go single-line struct syntax)
        raw_lines = body.strip().splitlines()
        tokens_list = []
        for raw_line in raw_lines:
            for part in raw_line.split(";"):
                part = part.strip()
                if part:
                    tokens_list.append(part)
        base_types_found = []
        extra = []
        for line in tokens_list:
            # Skip comments
            if line.startswith("//"):
                continue
            # An embedded type has no field name — just a type identifier
            token = line.split()[0] if line.split() else ""
            if token in BASE_TYPE_MAP:
                base_types_found.append(token)
            elif token in STANDARD_FIELDS:
                pass  # standard, not extra
            elif token and not token.startswith("//"):
                extra.append(line)

        # Priority: specific bases (FlatDispatch/Line/Manifest/etc.) over AndroidRule
        base_type = None
        for bt in base_types_found:
            if bt == "AndroidRule":
                # AndroidRule is lowest priority — only use if no other base
                if base_type is None:
                    base_type = bt
            else:
                base_type = bt

        if base_type:
            structs[struct_name] = {
                "base_type": base_type,
                "interface": BASE_TYPE_MAP[base_type],
                "extra_fields": extra,
                "file": filepath,
            }
    return structs


def parse_register_calls(content: str) -> list:
    """Extract Register() calls from init() functions.

    Returns list of dicts with struct_type, rule_name, rule_set, severity, desc,
    and the raw register line.
    """
    results = []
    # Find init() function bodies
    init_pattern = re.compile(r"func\s+init\(\)\s*\{", re.MULTILINE)
    for init_match in init_pattern.finditer(content):
        start = init_match.end()
        # Walk forward to find matching closing brace
        depth = 1
        pos = start
        while pos < len(content) and depth > 0:
            if content[pos] == "{":
                depth += 1
            elif content[pos] == "}":
                depth -= 1
            pos += 1
        init_body = content[start : pos - 1]

        # Find Register/RegisterManifest/RegisterGradle/RegisterResource calls
        reg_pattern = re.compile(r"Register(?:Manifest|Gradle|Resource)?\(", re.MULTILINE)
        for reg_match in reg_pattern.finditer(init_body):
            call_start = reg_match.start()
            # Find matching paren
            depth = 1
            p = reg_match.end()
            while p < len(init_body) and depth > 0:
                if init_body[p] == "(":
                    depth += 1
                elif init_body[p] == ")":
                    depth -= 1
                p += 1
            call_text = init_body[call_start:p]

            # Extract struct type: &FooRule{...}
            st_match = re.search(r"&(\w+Rule\w*)\{", call_text)
            if not st_match:
                continue
            struct_type = st_match.group(1)

            # Extract BaseRule fields (standard pattern)
            name_match = re.search(r'RuleName:\s*"([^"]*)"', call_text)
            set_match = re.search(r'RuleSetName:\s*"([^"]*)"', call_text)
            sev_match = re.search(r'Sev:\s*"([^"]*)"', call_text)
            desc_match = re.search(r'Desc:\s*"([^"]*)"', call_text)

            # Fallback for AndroidRule helper patterns like alcRule("Name", ...)
            rule_name = name_match.group(1) if name_match else ""
            if not rule_name:
                alc_match = re.search(
                    r'(?:alcRule|aluRule|alpRule|aliRule|alsRule)\(\s*"([^"]*)"',
                    call_text,
                )
                if alc_match:
                    rule_name = alc_match.group(1)

            results.append({
                "struct_type": struct_type,
                "rule_name": rule_name,
                "rule_set": set_match.group(1) if set_match else "",
                "severity": sev_match.group(1) if sev_match else "",
                "description": desc_match.group(1) if desc_match else "",
                "raw": call_text.strip(),
            })
    return results


def detect_optional_interfaces(content: str) -> dict:
    """Detect which struct types implement optional interfaces and check methods."""
    interfaces = defaultdict(set)
    for pattern_str, iface in [
        (r"func\s+\(r\s+\*(\w+)\)\s+Confidence\(\)\s+float64", "Confidence"),
        (r"func\s+\(r\s+\*(\w+)\)\s+FixLevel\(\)", "FixLevel"),
        (r"func\s+\(r\s+\*(\w+)\)\s+OracleFilter\(\)", "OracleFilter"),
        (r"func\s+\(r\s+\*(\w+)\)\s+SetResolver\(", "SetResolver"),
        (r"func\s+\(r\s+\*(\w+)\)\s+CheckFlatNode\(", "CheckFlatNode"),
        (r"func\s+\(r\s+\*(\w+)\)\s+CheckLines\(", "CheckLines"),
        (r"func\s+\(r\s+\*(\w+)\)\s+CheckManifest\(", "CheckManifest"),
        (r"func\s+\(r\s+\*(\w+)\)\s+CheckResources\(", "CheckResources"),
        (r"func\s+\(r\s+\*(\w+)\)\s+CheckGradle\(", "CheckGradle"),
        (r"func\s+\(r\s+\*(\w+)\)\s+CheckIcons\(", "CheckIcons"),
        (r"func\s+\(r\s+\*(\w+)\)\s+NodeTypes\(\)", "NodeTypes"),
    ]:
        for m in re.finditer(pattern_str, content):
            interfaces[m.group(1)].add(iface)
    return interfaces


def scan_rules_dir(rules_dir: str) -> list:
    """Scan all .go files and build a list of RuleInfo objects."""
    rules_dir = os.path.abspath(rules_dir)
    all_rules = []
    # Collect all file contents for cross-file interface detection
    # (e.g. fixlevel.go, oracle_filter_samples.go define methods on
    # structs declared in other files)
    all_content = {}
    go_files = sorted(Path(rules_dir).glob("*.go"))
    for f in go_files:
        if f.name.endswith("_test.go"):
            continue
        all_content[str(f)] = f.read_text()

    # First pass: collect all optional interface implementations across all files
    global_interfaces = defaultdict(set)
    for filepath, content in all_content.items():
        for struct_type, ifaces in detect_optional_interfaces(content).items():
            global_interfaces[struct_type].update(ifaces)

    # Second pass: parse structs and register calls per file
    for filepath, content in all_content.items():
        structs = parse_struct_types(content, filepath)
        register_calls = parse_register_calls(content)

        for reg in register_calls:
            st = reg["struct_type"]
            if st not in structs:
                # Struct might be defined in another file; skip or mark unknown
                info = RuleInfo(
                    name=reg["rule_name"],
                    struct_type=st,
                    base_type="unknown",
                    interface="Unknown",
                    file=os.path.basename(filepath),
                    rule_set=reg["rule_set"],
                    severity=reg["severity"],
                    description=reg["description"],
                    register_line=reg["raw"],
                )
            else:
                s = structs[st]
                info = RuleInfo(
                    name=reg["rule_name"],
                    struct_type=st,
                    base_type=s["base_type"],
                    interface=s["interface"],
                    file=os.path.basename(filepath),
                    extra_fields=s["extra_fields"],
                    rule_set=reg["rule_set"],
                    severity=reg["severity"],
                    description=reg["description"],
                    register_line=reg["raw"],
                )

            # Apply global interface detection
            ifaces = global_interfaces.get(st, set())
            info.has_confidence = "Confidence" in ifaces
            info.has_fix_level = "FixLevel" in ifaces
            info.has_oracle_filter = "OracleFilter" in ifaces
            info.has_set_resolver = "SetResolver" in ifaces
            info.has_check_flat_node = "CheckFlatNode" in ifaces
            info.has_check_lines = "CheckLines" in ifaces
            info.has_node_types = "NodeTypes" in ifaces

            # Refine LegacyAndroid rules based on actual methods implemented
            if info.interface == "LegacyAndroid":
                if "CheckManifest" in ifaces:
                    info.interface = "ManifestRule"
                elif "CheckResources" in ifaces:
                    info.interface = "ResourceRule"
                elif "CheckGradle" in ifaces:
                    info.interface = "GradleRule"
                elif "CheckIcons" in ifaces:
                    info.interface = "IconRule"
                elif "CheckFlatNode" in ifaces:
                    info.interface = "LegacyAndroid(FlatDispatch)"
                elif "CheckLines" in ifaces:
                    info.interface = "LegacyAndroid(Line)"
                else:
                    info.interface = "LegacyAndroid(StubOnly)"

            all_rules.append(info)

    return all_rules


def print_report(rules: list) -> None:
    """Print the diagnostic report."""
    print("=" * 72)
    print("  v1 -> v2 Rule Migration Diagnostic Report")
    print("=" * 72)
    print()

    # --- Summary by interface type ---
    by_interface = defaultdict(list)
    for r in rules:
        by_interface[r.interface].append(r)

    print("--- Rules by Interface Type ---")
    print()
    for iface in sorted(by_interface.keys()):
        count = len(by_interface[iface])
        print(f"  {iface:<25s} {count:>4d} rules")
    print(f"  {'TOTAL':<25s} {len(rules):>4d} rules")
    print()

    # --- Easy migrations (no extra fields, no SetResolver) ---
    easy = [r for r in rules if not r.extra_fields and not r.has_set_resolver]
    needs_adapter = [r for r in rules if r.extra_fields and not r.has_set_resolver]
    type_aware = [r for r in rules if r.has_set_resolver]

    print("--- Migration Difficulty ---")
    print()
    print(f"  Easy (no extra fields):       {len(easy):>4d} rules")
    print(f"  Adapter needed (extra fields): {len(needs_adapter):>4d} rules")
    print(f"  TypeAware (SetResolver):       {len(type_aware):>4d} rules")
    print()

    # --- Optional interface breakdown ---
    with_confidence = [r for r in rules if r.has_confidence]
    with_fix_level = [r for r in rules if r.has_fix_level]
    with_oracle = [r for r in rules if r.has_oracle_filter]

    print("--- Optional Interface Providers ---")
    print()
    print(f"  Confidence():    {len(with_confidence):>4d} rules")
    print(f"  FixLevel():      {len(with_fix_level):>4d} rules")
    print(f"  OracleFilter():  {len(with_oracle):>4d} rules")
    print(f"  SetResolver():   {len(type_aware):>4d} rules")
    print()

    # --- Easy rules list ---
    print("--- Easy Rules (can become inline v2.Rule) ---")
    print()
    for iface in sorted(by_interface.keys()):
        iface_easy = [r for r in by_interface[iface] if r in easy]
        if not iface_easy:
            continue
        print(f"  [{iface}] ({len(iface_easy)} rules)")
        for r in sorted(iface_easy, key=lambda x: x.name):
            extras = []
            if r.has_confidence:
                extras.append("Confidence")
            if r.has_fix_level:
                extras.append("FixLevel")
            if r.has_oracle_filter:
                extras.append("OracleFilter")
            suffix = f"  +{','.join(extras)}" if extras else ""
            print(f"    {r.name:<50s} ({r.file}){suffix}")
        print()

    # --- Rules needing adapter (have extra config fields) ---
    print("--- Rules with Extra Fields (need adapter or struct preservation) ---")
    print()
    for iface in sorted(by_interface.keys()):
        iface_adapter = [r for r in by_interface[iface] if r in needs_adapter]
        if not iface_adapter:
            continue
        print(f"  [{iface}] ({len(iface_adapter)} rules)")
        for r in sorted(iface_adapter, key=lambda x: x.name):
            fields_str = ", ".join(r.extra_fields)
            print(f"    {r.name:<50s} fields: {fields_str}")
        print()

    # --- TypeAware rules ---
    if type_aware:
        print("--- TypeAware Rules (need NeedsResolver capability) ---")
        print()
        for r in sorted(type_aware, key=lambda x: x.name):
            extras = []
            if r.has_confidence:
                extras.append("Confidence")
            if r.has_fix_level:
                extras.append("FixLevel")
            if r.has_oracle_filter:
                extras.append("OracleFilter")
            suffix = f"  +{','.join(extras)}" if extras else ""
            extra_str = f"  fields: {', '.join(r.extra_fields)}" if r.extra_fields else ""
            print(f"    {r.name:<50s} ({r.interface}){suffix}{extra_str}")
        print()

    # --- Per-file summary ---
    by_file = defaultdict(list)
    for r in rules:
        by_file[r.file].append(r)

    print("--- Per-File Summary ---")
    print()
    print(f"  {'File':<45s} {'Total':>5s} {'Easy':>5s} {'Adapt':>5s} {'TypeAw':>6s}")
    print(f"  {'-'*45:<45s} {'-----':>5s} {'-----':>5s} {'-----':>5s} {'------':>6s}")
    for fname in sorted(by_file.keys()):
        file_rules = by_file[fname]
        n_easy = sum(1 for r in file_rules if r in easy)
        n_adapt = sum(1 for r in file_rules if r in needs_adapter)
        n_type = sum(1 for r in file_rules if r in type_aware)
        print(f"  {fname:<45s} {len(file_rules):>5d} {n_easy:>5d} {n_adapt:>5d} {n_type:>6d}")
    print()


def generate_v2_code(rules: list, target_file: str) -> None:
    """Generate v2 registration code for easy rules in the given file."""
    basename = os.path.basename(target_file)
    file_rules = [r for r in rules if r.file == basename]

    if not file_rules:
        print(f"No rules found in {basename}", file=sys.stderr)
        sys.exit(1)

    easy = [r for r in file_rules if not r.extra_fields and not r.has_set_resolver]
    hard = [r for r in file_rules if r.extra_fields or r.has_set_resolver]

    if not easy:
        print(f"No easy-migration rules in {basename}. All {len(file_rules)} rules "
              f"have extra fields or SetResolver.", file=sys.stderr)
        sys.exit(1)

    print(f"// v2 registrations for {basename}")
    print(f"// Generated by scripts/migrate_to_v2.py")
    print(f"// {len(easy)} easy rules, {len(hard)} skipped (need manual migration)")
    print()
    print("func init() {")

    for r in easy:
        adapter_fn = _adapter_func(r.interface)
        if not adapter_fn:
            print(f"\t// SKIP: {r.name} — unknown interface {r.interface}")
            continue

        # Build the v2.Register call
        opts = []
        if r.has_confidence:
            opts.append(f"v2.WithConfidence(r.Confidence())")
        if r.has_fix_level:
            opts.append(f"v2.WithFixLevel(r.FixLevel())")
        if r.has_oracle_filter:
            opts.append(f"v2.WithOracleFilter(r.OracleFilter())")

        print(f"\t// {r.name}")
        print(f"\t{{")
        print(f'\t\tr := &{r.struct_type}{{BaseRule: BaseRule{{RuleName: "{r.name}", '
              f'RuleSetName: "{r.rule_set}", Sev: "{r.severity}", '
              f'Desc: "{r.description}"}}}}')
        if opts:
            opts_str = ", ".join(opts)
            print(f"\t\tv2.Register({adapter_fn}, {opts_str})")
        else:
            print(f"\t\tv2.Register({adapter_fn})")
        print(f"\t}}")

    if hard:
        print()
        print(f"\t// --- {len(hard)} rules need manual migration ---")
        for r in hard:
            reason = []
            if r.extra_fields:
                reason.append(f"extra fields: {', '.join(r.extra_fields)}")
            if r.has_set_resolver:
                reason.append("TypeAware (SetResolver)")
            print(f"\t// {r.name}: {'; '.join(reason)}")

    print("}")


def _adapter_func(interface: str) -> str:
    """Return the v2 adapter function call template for a given interface."""
    adapters = {
        "FlatDispatchRule": 'v2.AdaptFlatDispatch(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.NodeTypes(), r.CheckFlatNode)',
        "LineRule": 'v2.AdaptLine(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.CheckLines)',
        "ManifestRule": 'v2.AdaptManifest(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.CheckManifest)',
        "ResourceRule": 'v2.AdaptResource(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.CheckResources)',
        "LayoutResourceRule": 'v2.AdaptResource(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.CheckResources)',
        "GradleRule": 'v2.AdaptGradle(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.CheckGradle)',
        "IconRule": 'v2.AdaptIcon(r.RuleName, r.RuleSetName, r.Desc, v2.Severity(r.Sev), r.CheckIcons)',
    }
    return adapters.get(interface, "")


def main():
    parser = argparse.ArgumentParser(
        description="Diagnostic tool for v1 -> v2 rule registration migration."
    )
    parser.add_argument(
        "--generate",
        metavar="FILE.go",
        help="Generate v2 registration code for easy rules in the given file.",
    )
    parser.add_argument(
        "--rules-dir",
        default=RULES_DIR,
        help="Path to internal/rules/ directory (default: auto-detected).",
    )
    args = parser.parse_args()

    rules_dir = os.path.abspath(args.rules_dir)
    if not os.path.isdir(rules_dir):
        print(f"Rules directory not found: {rules_dir}", file=sys.stderr)
        sys.exit(1)

    rules = scan_rules_dir(rules_dir)

    if not rules:
        print("No rules found.", file=sys.stderr)
        sys.exit(1)

    if args.generate:
        generate_v2_code(rules, args.generate)
    else:
        print_report(rules)


if __name__ == "__main__":
    main()
