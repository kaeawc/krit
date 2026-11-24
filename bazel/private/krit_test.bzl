"""krit_test: a test that runs the prebuilt krit over Kotlin and Java sources."""

load("@bazel_skylib//lib:shell.bzl", "shell")

_SOURCE_EXTENSIONS = ["kt", "kts", "java"]

KritSourcesInfo = provider(
    doc = "Kotlin and Java sources a target lists in its srcs.",
    fields = {"srcs": "depset of File"},
)

def _krit_sources_aspect_impl(_target, ctx):
    srcs = []
    if hasattr(ctx.rule.files, "srcs"):
        srcs = [f for f in ctx.rule.files.srcs if f.extension in _SOURCE_EXTENSIONS]
    return [KritSourcesInfo(srcs = depset(srcs))]

# Not transitive: each target's own srcs, so a krit_test over a library does
# not also lint its dependencies.
_krit_sources_aspect = aspect(
    implementation = _krit_sources_aspect_impl,
    doc = "Collects the .kt, .kts and .java files in a target's srcs attribute.",
)

# Runs from the test's runfiles root (the main repository's directory), which
# is where `bazel test` and `bazel run` start it; every path is relative to it.
# Per-user and per-project state goes to scratch directories so the test
# never writes into the runfiles tree or the user's home, and krit neither
# starts a daemon, downloads the krit-fir jar nor launches a JVM.
_SCRIPT = """#!/usr/bin/env bash
set -euo pipefail
scratch="${{TEST_TMPDIR:-$(mktemp -d)}}/krit"
export HOME="$scratch/home" TMPDIR="$scratch/tmp"
export KRIT_STATE_DIR="$scratch/state" KRIT_HOME="$scratch/krit-home"
export KRIT_BUILDID_CACHE_BASE="$scratch/buildid"
export KRIT_NO_DAEMON_AUTOSTART=1 KRIT_NO_JAR_DOWNLOAD=1
# A JAVA_HOME without a JDK keeps krit from running a JVM (the Java facts
# helper would otherwise use whichever javac the machine has), so every
# machine runs the same Go-only analysis.
export JAVA_HOME="$scratch/no-jdk"
mkdir -p "$HOME" "$TMPDIR" "$KRIT_STATE_DIR" "$KRIT_HOME" "$KRIT_BUILDID_CACHE_BASE" "$JAVA_HOME"
rc=0
out=$({krit} --no-daemon --no-cache --no-fir --no-type-oracle --reproducible \\
  --base-path "$PWD" {flags} "$@" -- {files} 2>&1) || rc=$?
printf '%s\\n' "$out"
# krit skips files under test-data and test-resource directories (testdata/,
# src/test/resources/, ...) and exits 0 when nothing is left; do not pass a
# test that analyzed nothing.
if [ "$rc" -eq 0 ] && [[ "$out" == *"No Kotlin, Java, or Android project files found"* ]]; then
  echo "krit_test: krit analyzed none of the sources; it skips test-data and test-resource directories" >&2
  exit 1
fi
exit "$rc"
"""

def _krit_test_impl(ctx):
    srcs = depset(
        [f for f in ctx.files.srcs if f.extension in _SOURCE_EXTENSIONS],
        transitive = [t[KritSourcesInfo].srcs for t in ctx.attr.targets],
    ).to_list()
    if not srcs:
        fail("krit_test %s: no .kt, .kts or .java files in srcs or targets" % ctx.label)

    flags = ["--format", ctx.attr.format]
    inputs = list(srcs)
    if ctx.file.config:
        flags += ["--config", ctx.file.config.short_path]
        inputs.append(ctx.file.config)
    if ctx.file.baseline:
        flags += ["--baseline", ctx.file.baseline.short_path]
        inputs.append(ctx.file.baseline)

    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    ctx.actions.write(
        output = script,
        content = _SCRIPT.format(
            krit = shell.quote(ctx.executable._krit.short_path),
            flags = " ".join([shell.quote(f) for f in flags]),
            files = " ".join([shell.quote(f.short_path) for f in srcs]),
        ),
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = inputs + ctx.files.data)
    runfiles = runfiles.merge(ctx.attr._krit[DefaultInfo].default_runfiles)
    for d in ctx.attr.data:
        runfiles = runfiles.merge(d[DefaultInfo].default_runfiles)
    return [DefaultInfo(executable = script, runfiles = runfiles)]

krit_test = rule(
    implementation = _krit_test_impl,
    test = True,
    doc = """Runs krit over Kotlin and Java sources; the test fails when krit reports findings.

    load("@krit//:defs.bzl", "krit_test")

    krit_test(
        name = "lint",
        targets = [":lib"],  # kt_jvm_library, kt_android_library, java_library, ...
        config = "//:krit.yml",
    )

Sources come from `srcs` and from the `srcs` attribute of each target in
`targets`. The run is in-process and Go-only (no daemon, cache, FIR checker, type
oracle or Java facts helper), so it needs no JDK or network and gives the same
result on every machine, which Bazel caches on the sources and config. Reported paths are relative to the workspace root.
Extra krit flags go in `args`; they come before the files.
Linux and macOS only: the test is a shell script.
""",
    attrs = {
        "srcs": attr.label_list(
            allow_files = [".kt", ".kts", ".java"],
            doc = "Kotlin and Java files to analyze.",
        ),
        "targets": attr.label_list(
            aspects = [_krit_sources_aspect],
            doc = "Targets whose own srcs (not their dependencies') are analyzed, e.g. kt_jvm_library.",
        ),
        "config": attr.label(
            allow_single_file = True,
            doc = "krit.yml to use (--config).",
        ),
        "baseline": attr.label(
            allow_single_file = True,
            doc = "Baseline file of known findings to suppress (--baseline).",
        ),
        "format": attr.string(
            default = "plain",
            doc = "Report format krit prints to the test log (--format).",
        ),
        "data": attr.label_list(
            allow_files = True,
            doc = "Extra files the test's args refer to, e.g. args = [\"--baseline\", \"$(rootpath :old.xml)\"].",
        ),
        "_krit": attr.label(
            default = Label("//:krit"),
            executable = True,
            cfg = "target",
        ),
    },
)
