"""krit_test: a test that runs the prebuilt krit over Kotlin and Java sources."""

load("@bazel_skylib//lib:shell.bzl", "shell")
load("@rules_java//java/common:java_common.bzl", "java_common")
load("@rules_java//java/common:java_info.bzl", "JavaInfo")
load(":kotlin.bzl", "KT_TOOLCHAIN")
load(":model.bzl", "compile_classpath", "krit_model_json", "krit_source_set", "target_source_set")

_SOURCE_EXTENSIONS = ["kt", "kts", "java"]

KritSourcesInfo = provider(
    doc = "Kotlin and Java sources a target lists in its srcs, and the jars it compiles against.",
    fields = {
        "srcs": "depset of File",
        "source_set": "dict: the target's krit model source set (classpath, generated srcjars, Kotlin settings), with runfiles paths",
        "model_inputs": "depset of File: the files source_set names",
    },
)

def _krit_sources_aspect_impl(target, ctx):
    srcs = []
    srcjars = []
    if hasattr(ctx.rule.files, "srcs"):
        srcs = [f for f in ctx.rule.files.srcs if f.extension in _SOURCE_EXTENSIONS]
        srcjars = [f for f in ctx.rule.files.srcs if f.extension == "srcjar"]
    model = target_source_set(ctx, target, path = _short_path, srcjars = srcjars)
    return [KritSourcesInfo(
        srcs = depset(srcs),
        source_set = model.source_set,
        model_inputs = model.inputs,
    )]

def _short_path(f):
    return f.short_path

# Not transitive: each target's own srcs, so a krit_test over a library does
# not also lint its dependencies.
_krit_sources_aspect = aspect(
    implementation = _krit_sources_aspect_impl,
    toolchains = [KT_TOOLCHAIN],
    doc = "Collects the .kt, .kts and .java files in a target's srcs attribute and its compile classpath.",
)

# Runs from the test's runfiles root (the main repository's directory), which
# is where `bazel test` and `bazel run` start it; every path is relative to it.
# Per-user and per-project state goes to scratch directories so the test
# never writes into the runfiles tree or the user's home, and krit never
# starts a daemon or downloads the krit-fir jar.
_SCRIPT = """#!/usr/bin/env bash
set -euo pipefail
scratch="${{TEST_TMPDIR:-$(mktemp -d)}}/krit"
export HOME="$scratch/home" TMPDIR="$scratch/tmp"
export KRIT_STATE_DIR="$scratch/state" KRIT_HOME="$scratch/krit-home"
export KRIT_BUILDID_CACHE_BASE="$scratch/buildid"
export KRIT_NO_DAEMON_AUTOSTART=1 KRIT_NO_JAR_DOWNLOAD=1
mkdir -p "$HOME" "$TMPDIR" "$KRIT_STATE_DIR" "$KRIT_HOME" "$KRIT_BUILDID_CACHE_BASE"
{setup}
rc=0
out=$({krit} --no-daemon --no-cache --reproducible \\
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

# Go-only: a JAVA_HOME without a JDK keeps krit from running a JVM (the Java
# facts helper would otherwise use whichever javac the machine has), so every
# machine runs the same analysis.
_GO_ONLY_SETUP = """\
export JAVA_HOME="$scratch/no-jdk"
mkdir -p "$JAVA_HOME"
"""

# Type-aware: the JDK of Bazel's Java runtime toolchain (in runfiles, or an
# absolute path for a java_runtime with an absolute java_home) and the pinned
# krit-fir jar from runfiles; the model names the compile classpath.
_TYPE_AWARE_SETUP = """\
java_home={java_home}
case "$java_home" in
  /*) export JAVA_HOME="$java_home" ;;
  *) export JAVA_HOME="$PWD/$java_home" ;;
esac
export KRIT_FIR_JAR="$PWD/"{fir_jar}
"""

def _krit_test_impl(ctx):
    infos = [t[KritSourcesInfo] for t in ctx.attr.targets]
    srcs = depset(
        [f for f in ctx.files.srcs if f.extension in _SOURCE_EXTENSIONS],
        transitive = [i.srcs for i in infos],
    ).to_list()
    if not srcs:
        fail("krit_test %s: no .kt, .kts or .java files in srcs or targets" % ctx.label)

    flags = ["--format", ctx.attr.format]
    inputs = list(srcs)
    transitive_inputs = []
    if ctx.file.config:
        flags += ["--config", ctx.file.config.short_path]
        inputs.append(ctx.file.config)
    if ctx.file.baseline:
        flags += ["--baseline", ctx.file.baseline.short_path]
        inputs.append(ctx.file.baseline)

    if ctx.attr.type_aware:
        runtime = ctx.attr._java_runtime[java_common.JavaRuntimeInfo]
        fir_jar = ctx.file._krit_fir_jar
        source_sets = [i.source_set for i in infos]
        model_inputs = [i.model_inputs for i in infos]
        if ctx.attr.deps:
            # srcs outside any target compile against the deps' jars.
            classpath = depset(transitive = [compile_classpath(d, include_own = True) for d in ctx.attr.deps])
            source_sets.append(krit_source_set(
                label = ctx.label,
                classpath = [f.short_path for f in classpath.to_list()],
            ))
            model_inputs.append(classpath)
        model = ctx.actions.declare_file(ctx.label.name + ".krit-model/model.json")
        ctx.actions.write(
            output = model,
            content = krit_model_json(label = ctx.label, source_sets = source_sets),
        )

        # A FIR pass that cannot run fails the test instead of passing on a
        # Go-only result that Bazel would then cache.
        flags += [
            "--no-fir-daemon",
            "--fail-on-degraded",
            "--gradle-model",
            _dirname(model.short_path),
        ]
        inputs += [model, fir_jar]
        transitive_inputs += model_inputs + [runtime.files]
        setup = _TYPE_AWARE_SETUP.format(
            java_home = shell.quote(runtime.java_home_runfiles_path),
            fir_jar = shell.quote(fir_jar.short_path),
        )
    else:
        flags += ["--no-fir", "--no-type-oracle"]
        setup = _GO_ONLY_SETUP

    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    ctx.actions.write(
        output = script,
        content = _SCRIPT.format(
            setup = setup,
            krit = shell.quote(ctx.executable._krit.short_path),
            flags = " ".join([shell.quote(f) for f in flags]),
            files = " ".join([shell.quote(f.short_path) for f in srcs]),
        ),
        is_executable = True,
    )
    runfiles = ctx.runfiles(
        files = inputs + ctx.files.data,
        transitive_files = depset(transitive = transitive_inputs),
    )
    runfiles = runfiles.merge(ctx.attr._krit[DefaultInfo].default_runfiles)
    for d in ctx.attr.data:
        runfiles = runfiles.merge(d[DefaultInfo].default_runfiles)
    return [DefaultInfo(executable = script, runfiles = runfiles)]

def _dirname(path):
    return path.rpartition("/")[0] or "."

_ATTRS = {
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
}

# The type-aware variant is a rule of its own so that Go-only tests never
# depend on (and fetch) the JDK or the krit-fir jar.
_TYPE_AWARE_ATTRS = _ATTRS | {
    "deps": attr.label_list(
        providers = [JavaInfo],
        doc = "Extra libraries whose compile jars (and theirs, transitively) are on the classpath; for `srcs` that belong to no target in `targets`.",
    ),
    "type_aware": attr.bool(default = True),
    "_krit_fir_jar": attr.label(
        default = Label("//:krit_fir_jar"),
        allow_single_file = [".jar"],
    ),
    "_java_runtime": attr.label(
        default = Label("@bazel_tools//tools/jdk:current_java_runtime"),
        providers = [java_common.JavaRuntimeInfo],
    ),
}

_krit_go_test = rule(
    implementation = _krit_test_impl,
    test = True,
    attrs = _ATTRS | {"type_aware": attr.bool(default = False)},
)

_krit_type_aware_test = rule(
    implementation = _krit_test_impl,
    test = True,
    attrs = _TYPE_AWARE_ATTRS,
)

def krit_test(name, type_aware = False, deps = None, **kwargs):
    """Runs krit over Kotlin and Java sources; the test fails when krit reports findings.

        load("@krit//:defs.bzl", "krit_test")

        krit_test(
            name = "lint",
            targets = [":lib"],  # kt_jvm_library, kt_android_library, java_library, ...
            config = "//:krit.yml",
        )

    Sources come from `srcs` and from the `srcs` attribute of each target in
    `targets`. Reported paths are relative to the workspace root. Extra krit
    flags go in `args`; they come before the files. Linux and macOS only: the
    test is a shell script.

    By default the run is in-process and Go-only (no daemon, cache, FIR checker,
    type oracle or Java facts helper), so it needs no JDK or network and gives
    the same result on every machine, which Bazel caches on the sources and
    config.

    With `type_aware = True` krit also runs its FIR checker and type oracle on
    the JVM, against the classpath the targets compile with: their deps' compile
    jars (header or ABI jars) and, for Kotlin targets, the Kotlin stdlib, plus
    the compile jars of `deps`. The JDK is Bazel's Java runtime toolchain and
    the krit-fir jar is the checksum-verified asset of the krit release, so the
    test still needs no network and Bazel caches it on all of those inputs. A
    FIR pass that cannot run fails the test (`--fail-on-degraded`). Each run
    starts a cold JVM: expect a few seconds more per test.

    Args:
      name: target name.
      type_aware: also run krit's FIR checker and type oracle with the
        targets' compile classpath, Bazel's Java runtime and the pinned
        krit-fir jar.
      deps: with type_aware, extra libraries whose compile jars (and
        theirs, transitively) are on the classpath; for `srcs` that belong
        to no target in `targets`.
      **kwargs: srcs, targets, config, baseline, format, data, and the
        common test attributes (args, tags, size, ...).
    """
    if type_aware:
        _krit_type_aware_test(name = name, deps = deps or [], **kwargs)
    else:
        if deps:
            fail("krit_test %s: deps is only used with type_aware = True" % name)
        _krit_go_test(name = name, **kwargs)
