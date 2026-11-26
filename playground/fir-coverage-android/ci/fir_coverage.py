#!/usr/bin/env python3
"""FIR coverage of the sample: parse krit's verbose FIR verdict, report, and gate.

    fir_coverage.py measure NAME OUT.json LOG...
        Sum every "verbose: FIR verdict:" line in the LOG files (one per krit
        run; per-project kritCheck runs give one each) into OUT.json.

    fir_coverage.py measure-sarif NAME OUT.json [--bazel-log LOG] SARIF...
        Sum runs[].properties.fir (authoritative, gated, excluded, topCauses)
        of krit SARIF reports, such as krit_aspect's per-target
        <target>.krit.sarif files, into OUT.json. A report without it counts
        as a run where FIR did not run. With --bazel-log, each "Running krit
        on //x failed" action in the log (no report, e.g. a degraded FIR
        pass) is listed as a checker error.

    fir_coverage.py report THRESHOLDS.json FILE...
        Print a Markdown table of the *.result.json files (from measure) and
        the *.check.json files ({"name": ..., "passed": bool}, written by the
        workflow), and exit 1 when a result is below its threshold, has more
        FIR checker errors or failed actions than its maxCheckerErrors
        (default 0), or a required check failed. Anything above its threshold is listed as a
        ratchet: raise the threshold in the same pull request.

    fir_coverage.py parity OUT.check.json --gradle SARIF --bazel SARIF...
        Compare the findings of the Gradle whole-build report (kritCheckAll)
        with those of krit_aspect's per-target reports, keyed by rule ID,
        sample-relative path and line (#1677), and write the findings-parity
        check: it passes when both report the same findings and there is at
        least one. The keys only one side reports are listed with it.

Coverage is authoritative / (authoritative + gated), where gated counts both
compile-error and generated-source gating; excluded files (scripts, files
outside a JVM source set) are not in the denominator. This matches
--min-fir-coverage (#1573).
"""

import json
import re
import sys

VERDICT = re.compile(
    r"verbose: FIR verdict: (\d+) authoritative files, (\d+) gated \(compiler error or crash\), "
    r"(\d+) excluded \(scripts or not in a JVM source set\), (\d+) rule errors \(checker threw; "
    r"Go kept for that rule and file\), (\d+) gated \(generated sources\)"
)
GATED = re.compile(r"verbose: FIR gated: (.+?): (.*)$")
CHECKER_ERROR = re.compile(r"verbose: FIR checker error: (.*)$")
SAMPLE_PREFIX = re.compile(r"^.*?/playground/fir-coverage-android/")


def measure(name, out_path, log_paths):
    totals = {"authoritative": 0, "gated": 0, "generatedGated": 0, "excluded": 0, "ruleErrors": 0}
    runs = 0
    gated_files = []
    errors = []
    for path in log_paths:
        with open(path, encoding="utf-8", errors="replace") as log:
            for line in log:
                match = VERDICT.search(line)
                if match:
                    runs += 1
                    auth, gated, excluded, rule_errors, generated = map(int, match.groups())
                    totals["authoritative"] += auth
                    totals["gated"] += gated
                    totals["excluded"] += excluded
                    totals["ruleErrors"] += rule_errors
                    totals["generatedGated"] += generated
                    continue
                match = GATED.search(line)
                if match:
                    gated_files.append({"file": SAMPLE_PREFIX.sub("", match.group(1)), "cause": match.group(2).strip()})
                    continue
                match = CHECKER_ERROR.search(line)
                if match:
                    errors.append(match.group(1).strip())
    checked = totals["authoritative"] + totals["gated"] + totals["generatedGated"]
    result = {
        "name": name,
        "firRuns": runs,
        **totals,
        "checked": checked,
        "coverage": round(totals["authoritative"] / checked, 4) if checked else None,
        "gatedFiles": gated_files,
        "checkerErrors": errors,
    }
    with open(out_path, "w", encoding="utf-8") as out:
        json.dump(result, out, indent=2)
        out.write("\n")
    print(json.dumps({k: v for k, v in result.items() if k not in ("gatedFiles", "checkerErrors")}))
    for error in errors:
        print(f"FIR checker error: {error}", file=sys.stderr)


FAILED_KRIT_ACTION = re.compile(r"Running krit on (//\S+?) failed")


def measure_sarif(name, out_path, sarif_paths, bazel_log=None):
    totals = {"authoritative": 0, "gated": 0, "generatedGated": 0, "excluded": 0, "ruleErrors": 0}
    runs = 0
    causes = []
    errors = []
    for path in sarif_paths:
        with open(path, encoding="utf-8") as f:
            log = json.load(f)
        found = False
        for run in log.get("runs", []):
            fir = (run.get("properties") or {}).get("fir")
            if fir is None:
                continue
            found = True
            runs += 1
            totals["authoritative"] += fir.get("authoritative", 0)
            totals["gated"] += fir.get("gated", 0)
            totals["excluded"] += fir.get("excluded", 0)
            for cause in fir.get("topCauses") or []:
                causes.append({"file": SAMPLE_PREFIX.sub("", path), "cause": f"{cause['cause']} x{cause['count']}"})
        if not found:
            errors.append(f"{SAMPLE_PREFIX.sub('', path)}: no FIR coverage in the report")
    if bazel_log:
        with open(bazel_log, encoding="utf-8", errors="replace") as log:
            for line in log:
                match = FAILED_KRIT_ACTION.search(line)
                if match:
                    errors.append(f"{match.group(1)}: Krit action failed (see the bazel log)")
    checked = totals["authoritative"] + totals["gated"]
    result = {
        "name": name,
        "firRuns": runs,
        **totals,
        "checked": checked,
        "coverage": round(totals["authoritative"] / checked, 4) if checked else None,
        "gatedFiles": causes,
        "checkerErrors": errors,
    }
    with open(out_path, "w", encoding="utf-8") as out:
        json.dump(result, out, indent=2)
        out.write("\n")
    print(json.dumps({k: v for k, v in result.items() if k not in ("gatedFiles", "checkerErrors")}))
    for error in errors:
        print(error, file=sys.stderr)


def percent(value):
    return "n/a" if value is None else f"{value * 100:.0f}%"


def report(thresholds_path, paths):
    with open(thresholds_path, encoding="utf-8") as f:
        config = json.load(f)
    thresholds = config["coverage"]
    required_checks = config["checks"]
    results = []
    checks = []
    for path in paths:
        with open(path, encoding="utf-8") as f:
            (checks if path.endswith(".check.json") else results).append(json.load(f))

    failures = []
    ratchets = []
    # A threshold without a result means a run did not happen; fail rather
    # than pass vacuously.
    for name in sorted(set(thresholds) - {r["name"] for r in results}):
        failures.append(f"{name}: no result")
    for name in sorted(set(required_checks) - {c["name"] for c in checks}):
        failures.append(f"{name}: no result")
    lines = [
        "| path | FIR runs | authoritative | gated (compile) | gated (generated) | excluded | coverage | threshold |",
        "| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |",
    ]
    for r in results:
        t = thresholds.get(r["name"])
        if t is None:
            failures.append(f"{r['name']}: no entry in {thresholds_path}")
            t = {}
        min_auth = t.get("minAuthoritative", 0)
        min_cov = t.get("minCoverage", 0.0)
        coverage = r["coverage"] if r["coverage"] is not None else 0.0
        # Failed Krit actions and FIR checker errors leave no coverage record,
        # so their files would silently drop out of the ratio. Allow only the
        # known ones (maxCheckerErrors, ratchet down).
        max_errors = t.get("maxCheckerErrors", 0)
        errors = len(r.get("checkerErrors", []))
        if errors > max_errors:
            failures.append(f"{r['name']}: {errors} FIR checker errors or failed actions, at most {max_errors} allowed")
        elif errors < max_errors:
            ratchets.append(f"{r['name']}: maxCheckerErrors {max_errors} -> {errors}")
        if r["authoritative"] < min_auth:
            failures.append(f"{r['name']}: {r['authoritative']} authoritative files, threshold {min_auth}")
        if coverage + 1e-9 < min_cov:
            failures.append(f"{r['name']}: coverage {percent(coverage)}, threshold {percent(min_cov)}")
        if r["authoritative"] > min_auth or coverage > min_cov + 1e-9:
            ratchets.append(
                f"{r['name']}: minAuthoritative {min_auth} -> {r['authoritative']}, "
                f"minCoverage {min_cov} -> {coverage}"
            )
        lines.append(
            f"| {r['name']} | {r['firRuns']} | {r['authoritative']} | {r['gated']} | {r['generatedGated']} "
            f"| {r['excluded']} | {percent(r['coverage'])} | >= {min_auth} files, >= {percent(min_cov)} |"
        )
    if checks:
        lines += ["", "| check | result | required |", "| --- | --- | --- |"]
    for c in checks:
        required = required_checks.get(c["name"])
        if required is None:
            failures.append(f"{c['name']}: no entry in {thresholds_path}")
            required = {}
        req = required.get("required", False)
        if req and not c["passed"]:
            failures.append(f"{c['name']}: failed")
        if c["passed"] and not req:
            ratchets.append(f"{c['name']}: passes; set required to true")
        lines.append(f"| {c['name']} | {'pass' if c['passed'] else 'fail'} | {'yes' if req else 'no'} |")
    print("\n".join(lines))
    for c in checks:
        for side in ("onlyGradle", "onlyBazel"):
            if c.get(side):
                print(f"\n<details><summary>{c['name']}: {len(c[side])} findings {side}</summary>\n")
                for key in c[side]:
                    print(f"- `{key}`")
                print("\n</details>")
    for r in results:
        for error in r.get("checkerErrors", []):
            print(f"\n**{r['name']}: FIR checker error:** {error}")
        if r["gatedFiles"]:
            print(f"\n<details><summary>{r['name']}: gated files</summary>\n")
            for g in r["gatedFiles"]:
                print(f"- `{g['file']}`: {g['cause']}")
            print("\n</details>")
    if ratchets:
        print("\nAbove threshold; raise these in ci/fir-coverage-thresholds.json:\n")
        for line in ratchets:
            print(f"- {line}")
    if failures:
        print("\nBelow threshold (fix the regression; never lower a threshold):\n")
        for line in failures:
            print(f"- {line}")
        return 1
    return 0


def sarif_findings(paths):
    """The (rule, sample-relative path, line) keys of the results in SARIF reports.

    SARIF rule IDs are <ruleSet>/<rule>; only the rule is kept, so the two
    reports agree on the key whatever rule-set spelling each uses. Results in
    generated or build-output files are left out: the two build systems put
    those in different places.
    """
    keys = set()
    for path in paths:
        with open(path, encoding="utf-8") as f:
            sarif = json.load(f)
        for run in sarif.get("runs", []):
            for result in run.get("results", []):
                locations = result.get("locations") or [{}]
                physical = locations[0].get("physicalLocation", {})
                uri = physical.get("artifactLocation", {}).get("uri", "")
                line = physical.get("region", {}).get("startLine", 0)
                rel = SAMPLE_PREFIX.sub("", uri.removeprefix("file://")).removeprefix("./")
                if rel.startswith(("bazel-", "external/")) or "/build/" in f"/{rel}":
                    continue
                keys.add((result.get("ruleId", "").rsplit("/", 1)[-1], rel, line))
    return keys


def parity(out_path, gradle_paths, bazel_paths):
    gradle = sarif_findings(gradle_paths)
    bazel = sarif_findings(bazel_paths)
    fmt = lambda keys: [f"{rule} {path}:{line}" for rule, path, line in sorted(keys)]
    result = {
        "name": "findings-parity",
        "passed": bool(gradle) and gradle == bazel,
        "matched": len(gradle & bazel),
        "onlyGradle": fmt(gradle - bazel),
        "onlyBazel": fmt(bazel - gradle),
    }
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(result, f, indent=2)
    return result


def main(argv):
    if len(argv) >= 4 and argv[1] == "measure":
        measure(argv[2], argv[3], argv[4:])
        return 0
    if len(argv) >= 4 and argv[1] == "measure-sarif":
        rest = argv[4:]
        bazel_log = None
        if len(rest) >= 2 and rest[0] == "--bazel-log":
            bazel_log, rest = rest[1], rest[2:]
        measure_sarif(argv[2], argv[3], rest, bazel_log)
        return 0
    if len(argv) >= 3 and argv[1] == "report":
        return report(argv[2], argv[3:])
    if len(argv) >= 6 and argv[1] == "parity" and argv[3] == "--gradle" and "--bazel" in argv[4:]:
        split = argv.index("--bazel", 4)
        parity(argv[2], argv[4:split], argv[split + 1:])
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
