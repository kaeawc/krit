"""krit_aspect: krit with FIR as one cacheable, remotely executable action per target.

    bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit

writes `<target>.krit.sarif` (and a plain-text report) for every Kotlin or
Java target. With `--@krit//:fail_on_findings`, a validation action fails
the build when a target has findings.
"""

load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")
load("@rules_java//java/common:java_common.bzl", "java_common")
load("@rules_java//java/common:java_info.bzl", "JavaInfo")
load(":model.bzl", "compile_classpath", "krit_model_json", "label_name")

_SOURCE_EXTENSIONS = ["kt", "kts", "java"]

KritReportInfo = provider(
    doc = "The krit reports of one target, written by krit_aspect.",
    fields = {
        "sarif": "File: the SARIF report",
        "plain": "File: the plain-text report",
        "status": "File: krit's exit status, 0 (no findings) or 1 (findings)",
    },
)

# Runs from the execution root. krit's per-user state goes to a scratch
# directory removed afterwards, so the action reads and writes only its
# declared inputs and outputs: no $HOME, no daemon, no network (the krit-fir
# jar is an input and downloads are off). krit exits 1 when it reports
# findings; that still succeeds here and is recorded for the validation.
_RUN = """\
set -euo pipefail
status="$KRIT_STATUS_FILE"
java_home="${KRIT_JAVA_HOME:-}"
fir_jar="${KRIT_FIR_JAR_PATH:-}"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
export HOME="$scratch/home" TMPDIR="$scratch/tmp"
export KRIT_STATE_DIR="$scratch/state" KRIT_HOME="$scratch/krit-home"
export KRIT_BUILDID_CACHE_BASE="$scratch/buildid"
export KRIT_NO_DAEMON_AUTOSTART=1 KRIT_NO_JAR_DOWNLOAD=1
mkdir -p "$HOME" "$TMPDIR" "$KRIT_STATE_DIR" "$KRIT_HOME" "$KRIT_BUILDID_CACHE_BASE"
case "$java_home" in
  "") export JAVA_HOME="$scratch/no-jdk"; mkdir -p "$JAVA_HOME" ;;
  /*) export JAVA_HOME="$java_home" ;;
  *) export JAVA_HOME="$PWD/$java_home" ;;
esac
if [ -n "$fir_jar" ]; then
  export KRIT_FIR_JAR="$PWD/$fir_jar"
fi
rc=0
"$@" || rc=$?
case "$rc" in
  0 | 1) echo "$rc" > "$status" ;;
  *) exit "$rc" ;;
esac
"""

_VALIDATE = """\
set -euo pipefail
if [ "$(cat "$1")" != 0 ]; then
  echo "krit reported findings in $3:" >&2
  cat "$2" >&2
  exit 1
fi
: > "$4"
"""

def _flag(ctx, attr):
    return getattr(ctx.attr, attr)[BuildSettingInfo].value

def _krit_aspect_impl(target, ctx):
    if JavaInfo not in target or not hasattr(ctx.rule.files, "srcs"):
        return []
    if "no-krit" in getattr(ctx.rule.attr, "tags", []):
        return []
    srcs = [f for f in ctx.rule.files.srcs if f.extension in _SOURCE_EXTENSIONS]
    if not srcs:
        return []
    srcjars = [f for f in ctx.rule.files.srcs if f.extension == "srcjar"]

    name = target.label.name
    sarif = ctx.actions.declare_file(name + ".krit.sarif")
    plain = ctx.actions.declare_file(name + ".krit.txt")
    status = ctx.actions.declare_file(name + ".krit.status")

    args = ctx.actions.args()
    env = {"KRIT_STATUS_FILE": status.path, "LC_ALL": "C.UTF-8"}
    inputs = list(srcs)
    transitive = []
    tools = [ctx.attr._krit[DefaultInfo].files_to_run]
    krit_args = [
        "--no-daemon",
        "--no-cache",
        "--reproducible",
        "--base-path",
        ".",
        "--report",
        "sarif=" + sarif.path,
        "--report",
        "plain=" + plain.path,
    ]
    if _flag(ctx, "_fir"):
        runtime = ctx.attr._java_runtime[java_common.JavaRuntimeInfo]
        fir_jar = ctx.file._krit_fir_jar
        classpath = compile_classpath(target)
        model = ctx.actions.declare_file(name + ".krit-model/model.json")
        ctx.actions.write(
            output = model,
            content = krit_model_json(
                label = target.label,
                classpath = [f.path for f in classpath.to_list()],
                generated_source_jars = [f.path for f in srcjars],
                kind = "test" if getattr(ctx.rule.attr, "testonly", False) else "main",
            ),
        )
        env["KRIT_JAVA_HOME"] = runtime.java_home
        env["KRIT_FIR_JAR_PATH"] = fir_jar.path
        krit_args += [
            "--no-fir-daemon",
            "--fail-on-degraded",
            "--gradle-model",
            model.dirname,
        ]
        inputs += [model, fir_jar] + srcjars
        transitive += [classpath, runtime.files]
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

    args.add(ctx.executable._krit)
    args.add_all(krit_args)
    args.add("--")
    args.add_all(srcs)

    ctx.actions.run_shell(
        command = _RUN,
        arguments = [args],
        inputs = depset(inputs, transitive = transitive),
        outputs = [sarif, plain, status],
        tools = tools,
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
`srcs` (kt_jvm_library, kt_android_library, java_library, android_library,
their test variants, ...); a target tagged `no-krit` is skipped. It does not
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
- `--@krit//:fir=false`: Go-only analysis, without the JDK or krit-fir.
""",
    attrs = {
        "_krit": attr.label(
            default = Label("//:krit"),
            executable = True,
            cfg = "exec",
        ),
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
    },
)
