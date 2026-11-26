"""krit_project_report: krit's project-scope rules over a whole Bazel build.

krit classifies every rule as `file` scope (its findings for a file depend
only on that file and type information) or `project` scope (cross-file and
module rules such as DeadCode, rules over all parsed files, Android manifest
and resource rules); `krit --list-rules --format json` reports it. A
per-target krit_aspect action sees one target, so it runs only the file
rules (`--@krit//:scope=file`, the default): a project rule there would call
a declaration unused when its only caller is in another target.
krit_project_report runs the project rules once, over every source of its
targets and their dependencies in the main repository:

    load("@krit//:defs.bzl", "krit_project_report")

    krit_project_report(
        name = "krit_project",
        deps = ["//app:binary"],
    )

The action is Go-only (no JVM) and cacheable: Bazel re-runs it when one of
the collected sources, manifests, resources or the config changes.
"""

load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")
load(":android.bzl", "resource_dirs_build_file")

_SOURCE_EXTENSIONS = ["kt", "kts", "java"]

_KritProjectSourcesInfo = provider(
    doc = "A target's and its main-repository dependencies' sources, manifests and resources.",
    fields = {
        "srcs": "depset of File",
        "manifests": "depset of File",
        "resources": "depset of File",
    },
)

_DEP_ATTRS = ["deps", "exports", "runtime_deps", "associates", "srcs"]

def _project_sources_aspect_impl(target, ctx):
    srcs, manifests, resources = [], [], []

    # Only the main repository's own sources: external repositories are
    # dependencies, not part of the project.
    if not target.label.workspace_root:
        if hasattr(ctx.rule.files, "srcs"):
            srcs = [f for f in ctx.rule.files.srcs if f.is_source and f.extension in _SOURCE_EXTENSIONS]
        manifest = getattr(ctx.rule.file, "manifest", None)
        if manifest and manifest.is_source:
            manifests = [manifest]
        resources = [f for f in getattr(ctx.rule.files, "resource_files", []) if f.is_source]
    deps = []
    for attr in _DEP_ATTRS:
        for dep in getattr(ctx.rule.attr, attr, None) or []:
            if type(dep) == "Target" and _KritProjectSourcesInfo in dep:
                deps.append(dep[_KritProjectSourcesInfo])
    return [_KritProjectSourcesInfo(
        srcs = depset(srcs, transitive = [d.srcs for d in deps]),
        manifests = depset(manifests, transitive = [d.manifests for d in deps]),
        resources = depset(resources, transitive = [d.resources for d in deps]),
    )]

_project_sources_aspect = aspect(
    implementation = _project_sources_aspect_impl,
    attr_aspects = _DEP_ATTRS,
    doc = "Collects the main-repository sources, manifests and resources of a target and its dependencies.",
)

_VALIDATE = """\
set -euo pipefail
if [ "$(cat "$1")" != 0 ]; then
  echo "krit reported project findings in $3:" >&2
  cat "$2" >&2
  exit 1
fi
: > "$4"
"""

def _krit_project_report_impl(ctx):
    infos = [d[_KritProjectSourcesInfo] for d in ctx.attr.deps]
    srcs = depset(transitive = [i.srcs for i in infos]).to_list()
    manifests = depset(transitive = [i.manifests for i in infos]).to_list()
    resources = depset(transitive = [i.resources for i in infos]).to_list()
    extra = ctx.files.srcs
    if not srcs and not manifests and not resources and not extra:
        fail("krit_project_report %s: no sources, manifests or resources in deps" % ctx.label)

    sarif = ctx.actions.declare_file(ctx.label.name + ".krit.sarif")
    plain = ctx.actions.declare_file(ctx.label.name + ".krit.txt")
    status = ctx.actions.declare_file(ctx.label.name + ".krit.status")
    scan = srcs + manifests + extra
    inputs = srcs + manifests + resources + extra
    if resources:
        build_file = ctx.actions.declare_file(ctx.label.name + ".krit-project/BUILD.bazel")
        ctx.actions.write(
            output = build_file,
            content = resource_dirs_build_file(build_file.path, resources, path = lambda f: f.path),
        )
        scan.append(build_file)
        inputs.append(build_file)

    # `krit bazel-worker` runs one krit scan from a param file, with
    # scratch per-user state and, without a JAVA_HOME, no JVM (see
    # internal/cli/bazelworker); krit's exit 1 (findings) is recorded in the
    # status file, not a failure. A whole project's file list can be long.
    args = ctx.actions.args()
    args.use_param_file("@%s", use_always = True)
    args.set_param_file_format("multiline")
    args.add("--status-file", status)
    args.add_all([
        "--no-daemon",
        "--no-cache",
        "--no-fir",
        "--no-type-oracle",
        "--reproducible",
        "--only-scope",
        "project",
        "--base-path",
        ".",
        "--report",
        "sarif=" + sarif.path,
        "--report",
        "plain=" + plain.path,
    ])
    if ctx.file.config:
        args.add("--config", ctx.file.config)
        inputs.append(ctx.file.config)
    elif ctx.files._config:
        args.add("--config", ctx.files._config[0])
        inputs += ctx.files._config
    if ctx.file.baseline:
        args.add("--baseline", ctx.file.baseline)
        inputs.append(ctx.file.baseline)
    args.add("--")
    args.add_all(scan)

    ctx.actions.run(
        executable = ctx.executable._krit,
        arguments = ["bazel-worker", args],
        inputs = inputs,
        outputs = [sarif, plain, status],
        mnemonic = "KritProject",
        progress_message = "Running krit's project rules for %{label}",
        env = {"LC_ALL": "C.UTF-8"},
    )

    groups = {}
    fail_on_findings = ctx.attr.fail_on_findings
    if fail_on_findings == "auto":
        fail_on_findings = "true" if ctx.attr._fail_on_findings[BuildSettingInfo].value else "false"
    if fail_on_findings == "true":
        ok = ctx.actions.declare_file(ctx.label.name + ".krit.ok")
        ctx.actions.run_shell(
            command = _VALIDATE,
            arguments = [status.path, plain.path, str(ctx.label), ok.path],
            inputs = [status, plain],
            outputs = [ok],
            mnemonic = "KritValidate",
            progress_message = "Checking krit's project findings for %{label}",
        )
        groups["_validation"] = depset([ok])
    return [
        DefaultInfo(files = depset([sarif, plain])),
        OutputGroupInfo(krit = depset([sarif, plain]), **groups),
    ]

krit_project_report = rule(
    implementation = _krit_project_report_impl,
    doc = """Runs krit's project-scope rules over the sources of `deps` and their dependencies.

Writes `<name>.krit.sarif` and `<name>.krit.txt`. The sources are the
main repository's `srcs` (`.kt`, `.kts`, `.java`), `manifest` and
`resource_files` of `deps` and of everything they depend on through
`deps`, `exports`, `runtime_deps` and `associates`. Use it next to
krit_aspect, whose per-target actions run the file-scope rules.
""",
    attrs = {
        "deps": attr.label_list(
            aspects = [_project_sources_aspect],
            doc = "Top-level targets (binaries, tests, libraries) whose transitive main-repository sources form the project.",
        ),
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Extra files to scan, e.g. Gradle files or other sources outside the deps' graph.",
        ),
        "config": attr.label(
            allow_single_file = True,
            doc = "krit.yml (--config); defaults to --@krit//:config.",
        ),
        "baseline": attr.label(
            allow_single_file = True,
            doc = "Baseline file of known findings to suppress (--baseline).",
        ),
        "fail_on_findings": attr.string(
            default = "auto",
            values = ["auto", "true", "false"],
            doc = "Fail the build through a validation action when krit reports findings; auto follows --@krit//:fail_on_findings.",
        ),
        "_krit": attr.label(
            default = Label("//:krit"),
            executable = True,
            cfg = "exec",
        ),
        "_config": attr.label(
            default = Label("//:config"),
            allow_files = True,
        ),
        "_fail_on_findings": attr.label(
            default = Label("//:fail_on_findings"),
            providers = [BuildSettingInfo],
        ),
    },
)
