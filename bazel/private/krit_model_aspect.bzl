"""krit_model_aspect: a krit model of every Kotlin/Java target in the build.

    bazel run @krit//:export_model -- //...

builds `--output_groups=krit_model` with this aspect and collects the files
into `.krit/bazel-model/`, where `krit`, `krit-lsp` and `krit-mcp` find them
the way they find `.krit/gradle-model/` after `kritExportModel`: FIR scans
outside Bazel then get each target's full classpath, generated sources,
Kotlin settings and Android inputs.

Each file has one project per target, in the Gradle model's schema: the
target's label is the project path, its deps' labels are projectDeps, and
its source set lists the directories of its srcs. Paths are written with
placeholders, @@WORKSPACE@@ for source files of the main repository and
@@EXECROOT@@ for everything else (bazel-out, external), which export_model
replaces with absolute paths.
"""

load("@rules_java//java/common:java_info.bzl", "JavaInfo")
load(":android.bzl", "krit_android_sdk_aspect")
load(":kotlin.bzl", "KT_TOOLCHAIN")
load(":model.bzl", "label_name", "target_source_set")

_SOURCE_EXTENSIONS = ["kt", "kts", "java"]

WORKSPACE = "@@WORKSPACE@@/"
EXECROOT = "@@EXECROOT@@/"

def _model_path(f):
    if f.is_source and not f.owner.workspace_root:
        return WORKSPACE + f.path
    return EXECROOT + f.path

def _project_deps(rule_attr):
    deps = []
    for attr in ["deps", "exports", "associates"]:
        for dep in getattr(rule_attr, attr, None) or []:
            if type(dep) == "Target" and JavaInfo in dep and not dep.label.workspace_root:
                deps.append(label_name(dep.label))
    return sorted({d: True for d in deps}.keys())

def _krit_model_aspect_impl(target, ctx):
    if JavaInfo not in target or not hasattr(ctx.rule.files, "srcs"):
        return []
    if target.label.workspace_root:
        # Only the main repository's targets are analyzed; external ones
        # are on the classpath as jars.
        return []
    srcs = [f for f in ctx.rule.files.srcs if f.extension in _SOURCE_EXTENSIONS]
    srcjars = [f for f in ctx.rule.files.srcs if f.extension == "srcjar"]
    model = target_source_set(ctx, target, path = _model_path, srcjars = srcjars)
    if not srcs and not model.android.resource_files and not model.android.manifest:
        return []
    source_set = dict(model.source_set)
    source_set["sourceDirs"] = sorted({WORKSPACE + f.dirname: True for f in srcs if f.is_source}.keys())
    source_set["projectDeps"] = _project_deps(ctx.rule.attr)
    out = ctx.actions.declare_file(target.label.name + ".krit-model.json")
    ctx.actions.write(
        output = out,
        content = json.encode_indent({
            "schema": 2,
            "projects": [{
                "path": label_name(target.label),
                "dir": WORKSPACE + target.label.package if target.label.package else WORKSPACE[:-1],
                "sourceSets": [source_set],
            }],
        }, indent = "  ") + "\n",
    )

    # The jars and srcjars the model names are built with it, so the paths
    # exist when krit reads them.
    return [OutputGroupInfo(krit_model = depset([out], transitive = [model.inputs]))]

krit_model_aspect = aspect(
    implementation = _krit_model_aspect_impl,
    toolchains = [KT_TOOLCHAIN],
    requires = [krit_android_sdk_aspect],
    doc = """Writes `<target>.krit-model.json` (output group `krit_model`) for each Kotlin or Java target.

    bazel build //... --aspects=@krit//:defs.bzl%krit_model_aspect --output_groups=krit_model

`bazel run @krit//:export_model -- //...` builds them and installs them into
`.krit/bazel-model/` with absolute paths. The output group also builds the
jars and srcjars the models name.
""",
)
