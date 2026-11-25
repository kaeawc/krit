"""A test over the krit_aspect reports of one target."""

load("@krit//:defs.bzl", "KritReportInfo", "krit_aspect")

def _impl(ctx):
    info = ctx.attr.target[KritReportInfo]
    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    ctx.actions.write(
        output = script,
        is_executable = True,
        content = """#!/usr/bin/env bash
set -euo pipefail
status="$(cat {status})"
echo "krit status: $status"
cat {plain}
if [ "$status" != {want_status} ]; then
  echo "want krit status {want_status}" >&2
  exit 1
fi
for want in {expect}; do
  grep -qF -- "$want" {plain} || {{ echo "missing in the plain report: $want" >&2; exit 1; }}
done
grep -qE '"version": *"2.1.0"' {sarif} || {{ echo "not a SARIF 2.1.0 report: {sarif}" >&2; exit 1; }}
""".format(
            status = info.status.short_path,
            plain = info.plain.short_path,
            sarif = info.sarif.short_path,
            want_status = "1" if ctx.attr.expect else "0",
            expect = " ".join(["'%s'" % e for e in ctx.attr.expect]),
        ),
    )
    return [DefaultInfo(
        executable = script,
        runfiles = ctx.runfiles(files = [info.status, info.plain, info.sarif]),
    )]

krit_report_test = rule(
    implementation = _impl,
    test = True,
    attrs = {
        "target": attr.label(aspects = [krit_aspect], mandatory = True),
        "expect": attr.string_list(doc = "Substrings of the plain report; none means no findings."),
    },
)
