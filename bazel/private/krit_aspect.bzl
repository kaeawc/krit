"""krit_aspect: krit with FIR as one cacheable, remotely executable action per target.

    bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit

writes `<target>.krit.sarif` (and a plain-text report) for every Kotlin or
Java target. With `--@krit//:fail_on_findings`, a validation action fails
the build when a target has findings.
"""

load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")
load("@rules_java//java/common:java_common.bzl", "java_common")
load("@rules_java//java/common:java_info.bzl", "JavaInfo")
load(":android.bzl", "android_inputs", "krit_android_sdk_aspect", "resource_dirs_build_file", "sdk_level_args")
load(":kotlin.bzl", "KT_TOOLCHAIN")
load(":model.bzl", "krit_model_json", "label_name", "target_source_set")
load(":toolchain.bzl", "KRIT_TOOLCHAIN_TYPE")

_SOURCE_EXTENSIONS = ["kt", "kts", "java"]

KritReportInfo = provider(
    doc = "The krit reports of one target, written by krit_aspect.",
    fields = {
        "sarif": "File: the SARIF report",
        "plain": "File: the plain-text report",
        "status": "File: krit's exit status, 0 (no findings) or 1 (findings)",
    },
)

# The Krit action runs `krit bazel-worker` (internal/cli/bazelworker): with
# Bazel's persistent workers each worker keeps one warm krit-fir JVM across
# the actions it serves, otherwise (workers off, remote execution) it runs
# the scan with a one-shot JVM. Either way each action is a fresh krit
# process whose per-user and per-run state lives in scratch directories, so
# it reads and writes only its declared inputs and outputs: no $HOME, no
# network (the krit-fir jar is an input and downloads are off). krit exits 1
# when it reports findings; that still succeeds and is written to the status
# file for the validation. One krit-fir daemon serves every target with the
# same compiler settings, rebuilding its session for each action's sources
# and classpath, so a build of many targets pays for one JVM per worker;
# singleplex workers (one action each at a time) let it keep only its
# newest daemon, so it holds at most one JVM.
_WORKER_REQUIREMENTS = {
    "requires-worker-protocol": "json",
    "supports-workers": "1",
}

# The exec group of the Krit actions of large targets: with remote
# execution, a platform's `krit_large.<key>` exec_properties (a bigger
# worker pool, say) apply to them only. It has no constraints, so it
# resolves to the same execution platform as the default group; it exists
# only to carry those properties, since an aspect's actions never see a
# target's own exec_properties.
_LARGE_EXEC_GROUP = "krit_large"

# Local scheduling estimates (Bazel's resource_set, memory in MB) by the
# number of files a Krit action scans and reports on: small below 50,
# medium below _large_sources (--@krit//:large_target_sources, 400 by
# default), large from there. Generated srcjars are compiled too but not
# counted: their size is unknown at analysis time. With FIR the action runs
# krit and a krit-fir JVM; Go-only it runs krit alone. Measured with a
# stdlib-only classpath on 4 cores, a FIR action peaked at 366 MB (2
# files), 523 MB (50) and 642 MB (400) and used 2.5, 2.7 and 3.0 CPUs
# (mostly JVM startup and JIT for the small ones); Go-only it peaked at
# 33-59 MB. The memory estimates leave room for real classpaths. Bazel
# only accepts top-level functions here.
def _resources_fir_small(_os, _inputs):
    return {"cpu": 1, "memory": 512}

def _resources_fir_medium(_os, _inputs):
    return {"cpu": 2, "memory": 1024}

def _resources_fir_large(_os, _inputs):
    return {"cpu": 2, "memory": 2048}

def _resources_go_small(_os, _inputs):
    return {"cpu": 1, "memory": 128}

def _resources_go_medium(_os, _inputs):
    return {"cpu": 2, "memory": 256}

def _resources_go_large(_os, _inputs):
    return {"cpu": 4, "memory": 512}

_SMALL_SOURCES = 50

_RESOURCES = {
    (True, "small"): _resources_fir_small,
    (True, "medium"): _resources_fir_medium,
    (True, "large"): _resources_fir_large,
    (False, "small"): _resources_go_small,
    (False, "medium"): _resources_go_medium,
    (False, "large"): _resources_go_large,
}

def _size(count, large):
    if count >= large:
        return "large"
    if count >= _SMALL_SOURCES:
        return "medium"
    return "small"

_VALIDATE = """\
set -euo pipefail
if [ "$(cat "$1")" != 0 ]; then
  echo "krit reported findings in $3:" >&2
  cat "$2" >&2
  exit 1
fi
: > "$4"
"""

def _exec_path(f):
    return f.path

def _string(ctx, attr):
    return getattr(ctx.attr, attr)[BuildSettingInfo].value

def _flag(ctx, attr):
    return getattr(ctx.attr, attr)[BuildSettingInfo].value

def _krit_aspect_impl(target, ctx):
    if JavaInfo not in target or not hasattr(ctx.rule.files, "srcs"):
        return []
    if "no-krit" in getattr(ctx.rule.attr, "tags", []):
        return []
    srcs = [f for f in ctx.rule.files.srcs if f.extension in _SOURCE_EXTENSIONS]
    srcjars = [f for f in ctx.rule.files.srcs if f.extension == "srcjar"]

    # The target's own manifest and resources, for krit's Android rules.
    android = android_inputs(ctx, target)
    android_files = list(android.resource_files)
    if android.manifest:
        android_files.append(android.manifest)
    if not srcs and not android_files:
        return []

    name = target.label.name
    sarif = ctx.actions.declare_file(name + ".krit.sarif")
    plain = ctx.actions.declare_file(name + ".krit.txt")
    status = ctx.actions.declare_file(name + ".krit.status")

    args = ctx.actions.args()
    args.use_param_file("@%s", use_always = True)
    args.set_param_file_format("multiline")
    args.add("--status-file", status)
    env = {"LC_ALL": "C.UTF-8"}
    inputs = list(srcs)
    transitive = []
    krit_args = [
        "--no-daemon",
        "--no-cache",
        "--reproducible",
        "--base-path",
        ".",
    ]

    # File-scope rules only, by default: a project-scope rule (cross-file,
    # module, Android manifest and resource rules) sees one target here and
    # would report, say, a declaration as unused that another target uses.
    # krit_project_report runs those once over the whole build.
    if _string(ctx, "_scope") == "file":
        krit_args += ["--only-scope", "file"]
    krit_args += sdk_level_args(
        _flag(ctx, "_min_sdk"),
        _flag(ctx, "_target_sdk"),
        getattr(ctx.rule.attr, "manifest_values", None) or {},
    )
    krit_args += [
        "--report",
        "sarif=" + sarif.path,
        "--report",
        "plain=" + plain.path,
    ]
    if _flag(ctx, "_fir"):
        runtime = ctx.attr._java_runtime[java_common.JavaRuntimeInfo]
        fir_jar = ctx.file._krit_fir_jar
        model_set = target_source_set(ctx, target, path = _exec_path, srcjars = srcjars)
        model = ctx.actions.declare_file(name + ".krit-model/model.json")
        ctx.actions.write(
            output = model,
            content = krit_model_json(label = target.label, source_sets = [model_set.source_set]),
        )
        env["JAVA_HOME"] = runtime.java_home
        env["KRIT_FIR_JAR"] = fir_jar.path

        # No --no-fir-daemon: `krit bazel-worker` adds it unless it runs as
        # a persistent worker, whose krit-fir daemon serves every request.
        krit_args += [
            "--fail-on-degraded",
            "--gradle-model",
            model.dirname,
        ]
        inputs += [model, fir_jar]
        transitive += [model_set.inputs, runtime.files]
    else:
        krit_args += ["--no-fir", "--no-type-oracle"]

    config = ctx.files._config
    if config:
        krit_args += ["--config", config[0].path]
        inputs += config
    baseline = ctx.files._baseline
    if baseline:
        krit_args += ["--baseline", baseline[0].path]
        inputs += baseline

    scan = list(srcs)
    if android.manifest:
        scan.append(android.manifest)
    inputs += android_files
    if android.resource_files:
        build_file = ctx.actions.declare_file(name + ".krit-android/BUILD.bazel")
        ctx.actions.write(
            output = build_file,
            content = resource_dirs_build_file(build_file.path, android.resource_files, path = _exec_path),
        )
        inputs.append(build_file)
        scan.append(build_file)

    args.add_all(krit_args)
    args.add("--")
    args.add_all(scan)

    size = _size(len(scan), ctx.attr._large_sources[BuildSettingInfo].value)
    exec_group = _LARGE_EXEC_GROUP if size == "large" else None

    # The krit binary for the platform that runs this action's exec group.
    toolchains = ctx.exec_groups[exec_group].toolchains if exec_group else ctx.toolchains
    ctx.actions.run(
        executable = toolchains[KRIT_TOOLCHAIN_TYPE].krit,
        arguments = ["bazel-worker", args],
        execution_requirements = _WORKER_REQUIREMENTS if _flag(ctx, "_worker") else {},
        exec_group = exec_group,
        resource_set = _RESOURCES[(_flag(ctx, "_fir"), size)],
        inputs = depset(inputs, transitive = transitive),
        outputs = [sarif, plain, status],
        mnemonic = "Krit",
        progress_message = "Running krit on %{label}",
        env = env,
    )

    groups = {"krit": depset([sarif, plain])}
    if _flag(ctx, "_fail_on_findings"):
        ok = ctx.actions.declare_file(name + ".krit.ok")
        ctx.actions.run_shell(
            command = _VALIDATE,
            arguments = [status.path, plain.path, label_name(target.label), ok.path],
            inputs = [status, plain],
            outputs = [ok],
            mnemonic = "KritValidate",
            progress_message = "Checking krit findings of %{label}",
        )
        groups["_validation"] = depset([ok])
    return [
        OutputGroupInfo(**groups),
        KritReportInfo(sarif = sarif, plain = plain, status = status),
    ]

krit_aspect = aspect(
    implementation = _krit_aspect_impl,
    doc = """Runs krit over each Kotlin or Java target's own sources as one Bazel action.

    bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit

Applies to every target with `JavaInfo` and `.kt`, `.kts` or `.java` files in
`srcs`, or with a manifest or resource files of its own (kt_jvm_library,
kt_android_library, java_library, android_library, their test variants, ...);
a target tagged `no-krit` is skipped. Android targets compile against the
Android SDK toolchain's android.jar and their R class jars, and their manifest
and resources are scanned too. It does not
propagate to dependencies: each target is analyzed once, against its deps'
compile jars (header or ABI jars) rather than their sources, so a change
below re-runs only the targets whose classpath changed. `.srcjar` files in
`srcs` are compiled as generated-source context and not reported on.

The action's inputs are the sources, the classpath, krit, the krit-fir jar,
Bazel's Java runtime and the configured krit.yml and baseline; it needs no
network, `$HOME` or daemon, so Bazel caches it locally or remotely and can
execute it remotely. A FIR pass that cannot run fails the action
(`--fail-on-degraded`).

Output group `krit`: `<target>.krit.sarif` and `<target>.krit.txt`.

Flags:

- `--@krit//:config=//:krit.yml` and `--@krit//:baseline=//:baseline.xml`.
- `--@krit//:fail_on_findings`: a validation action fails the build when a
  target has findings (with the default `--run_validations`).
- `--@krit//:scope=all`: also run krit's project-scope rules in each
  target's action (the default `file` leaves them to krit_project_report).
- `--@krit//:fir=false`: Go-only analysis, without the JDK or krit-fir.
- `--@krit//:worker=false`: no persistent worker; every action starts its
  own krit-fir JVM (as with `--strategy=Krit=sandboxed` or remote execution).
  With workers, `--worker_max_instances=Krit=N` bounds the JVMs (one per
  worker) and `--worker_sandboxing` sandboxes them.
- `--@krit//:large_target_sources=N` (400): actions that scan at least N
  files run in the `krit_large` exec group, so an execution platform's
  `krit_large.<key>` exec_properties apply to them only.
""",
    toolchains = [KT_TOOLCHAIN, KRIT_TOOLCHAIN_TYPE],
    exec_groups = {_LARGE_EXEC_GROUP: exec_group(toolchains = [KRIT_TOOLCHAIN_TYPE])},
    requires = [krit_android_sdk_aspect],
    attrs = {
        "_krit_fir_jar": attr.label(
            default = Label("//:krit_fir_jar"),
            allow_single_file = [".jar"],
        ),
        "_java_runtime": attr.label(
            default = Label("@bazel_tools//tools/jdk:current_java_runtime"),
            cfg = "exec",
            providers = [java_common.JavaRuntimeInfo],
        ),
        "_config": attr.label(
            default = Label("//:config"),
            allow_files = True,
        ),
        "_baseline": attr.label(
            default = Label("//:baseline"),
            allow_files = True,
        ),
        "_fail_on_findings": attr.label(
            default = Label("//:fail_on_findings"),
            providers = [BuildSettingInfo],
        ),
        "_fir": attr.label(
            default = Label("//:fir"),
            providers = [BuildSettingInfo],
        ),
        "_worker": attr.label(
            default = Label("//:worker"),
            providers = [BuildSettingInfo],
        ),
        "_scope": attr.label(
            default = Label("//:scope"),
            providers = [BuildSettingInfo],
        ),
        "_large_sources": attr.label(
            default = Label("//:large_target_sources"),
            providers = [BuildSettingInfo],
        ),
        "_min_sdk": attr.label(
            default = Label("//:min_sdk_version"),
            providers = [BuildSettingInfo],
        ),
        "_target_sdk": attr.label(
            default = Label("//:target_sdk_version"),
            providers = [BuildSettingInfo],
        ),
    },
)
