"""The krit model of a Bazel target: what krit's FIR checker and type oracle compile it with.

krit reads the same JSON schema as the model krit-gradle-plugin exports
(internal/gradlemodel in the krit repository), so a Bazel target becomes one
project with one source set.
"""

load("@rules_java//java/common:java_info.bzl", "JavaInfo")
load(":android.bzl", "android_inputs")
load(":kotlin.bzl", "kotlin_settings")

# Schema 2 is the first with Kotlin compiler settings; krit accepts 1 and 2.
_SCHEMA = 2

def compile_classpath(target, include_own = False):
    """Returns the jars a target's own compilation sees, as a depset.

    That is JavaInfo.transitive_compile_time_jars: the deps' (and their
    exports') compile jars, which are header or ABI jars where the rules make
    them, and for rules_kotlin targets the Kotlin stdlib the toolchain adds.
    The target's own outputs are dropped unless include_own is set: krit
    analyzes its sources, so its own jar would declare every type twice.

    Args:
      target: a Target; one without JavaInfo has an empty classpath.
      include_own: keep the target's own compile jars (for a dependency).

    Returns:
      depset of File.
    """
    if JavaInfo not in target:
        return depset()
    info = target[JavaInfo]
    if include_own:
        return info.transitive_compile_time_jars
    own = {}
    for output in info.java_outputs:
        for jar in [output.class_jar, output.compile_jar]:
            if jar:
                own[jar] = True
    return depset([
        jar
        for jar in info.transitive_compile_time_jars.to_list()
        if jar not in own
    ])

def label_name(label):
    """Returns a label as //pkg:name (main repository) or @repo//pkg:name."""

    # "//pkg:name" for the main repository, "@repo//pkg:name" otherwise; never
    # the canonical "@@" form, which varies with the module graph.
    repo = "@" + label.repo_name if label.repo_name else ""
    return "%s//%s:%s" % (repo, label.package, label.name)

def krit_source_set(
        label,
        classpath,
        generated_source_jars = [],
        kind = "main",
        kotlin = None,
        jvm_target = "",
        platform = "jvm",
        boot_classpath = [],
        generated_classpath = []):
    """Returns one source set of a krit model: one Bazel target.

    Args:
      label: Label of the target; it names the source set.
      classpath: list of jar paths, relative to the directory krit runs in.
      generated_source_jars: list of .srcjar paths compiled as generated
        context and never reported on.
      kind: "main" or "test".
      kotlin: the target's Kotlin compiler settings (the model's `kotlin`
        object, see kotlin.bzl), or None.
      jvm_target: the JVM target, e.g. "17".
      platform: "jvm" or "android".
      boot_classpath: list of boot classpath jar paths (android.jar).
      generated_classpath: list of generated class jar paths (R jars).

    Returns:
      A dict in the model's sourceSets schema.
    """
    source_set = {
        "name": label_name(label),
        "kind": kind,
        "platform": platform,
        "variant": "",
        "sourceDirs": [],
        "generatedSourceDirs": [],
        "generatedSourceJars": generated_source_jars,
        "classpath": classpath,
        "generatedClasspath": generated_classpath,
        "bootClasspath": boot_classpath,
        "projectDeps": [],
        "jvmTarget": jvm_target,
    }
    if kotlin != None:
        source_set["kotlin"] = kotlin
    return source_set

def krit_model_json(label, source_sets):
    """Renders a one-project krit model.

    Args:
      label: Label of the rule or target the model is for; it names the
        project.
      source_sets: list of krit_source_set dicts.

    Returns:
      The model file's JSON text.
    """
    return json.encode_indent({
        "schema": _SCHEMA,
        "projects": [{
            "path": label_name(label),
            "dir": ".",
            "sourceSets": source_sets,
        }],
    }, indent = "  ") + "\n"

def target_source_set(ctx, target, path, srcjars = [], classpath = None):
    """Describes what one target compiles with, for krit's model.

    Args:
      ctx: aspect context (its toolchains include kotlin.bzl's KT_TOOLCHAIN).
      target: the analyzed target.
      path: function from File to the path krit sees (File.path in an
        action, File.short_path in runfiles).
      srcjars: the target's own .srcjar sources.
      classpath: depset of jars; defaults to compile_classpath(target).

    Returns:
      struct(source_set, inputs, android): the krit_source_set dict, a
      depset of the files it names, and the target's android_inputs.
    """
    if classpath == None:
        classpath = compile_classpath(target)
    kotlin = kotlin_settings(ctx, target)
    android = android_inputs(ctx, target)
    generated = srcjars + kotlin.generated_source_jars
    settings = None
    if kotlin.kotlin != None:
        settings = dict(kotlin.kotlin)
        settings["compilerPluginClasspath"] = [path(f) for f in kotlin.plugin_jars.to_list()]
    source_set = krit_source_set(
        label = target.label,
        classpath = [path(f) for f in classpath.to_list()],
        generated_source_jars = [path(f) for f in generated],
        kind = "test" if getattr(ctx.rule.attr, "testonly", False) else "main",
        kotlin = settings,
        jvm_target = kotlin.jvm_target,
        platform = "android" if android.is_android else "jvm",
        boot_classpath = [path(f) for f in android.boot_classpath],
        generated_classpath = [path(f) for f in android.r_jars.to_list()],
    )
    return struct(
        source_set = source_set,
        inputs = depset(
            generated + android.boot_classpath,
            transitive = [classpath, kotlin.plugin_jars, android.r_jars],
        ),
        android = android,
    )
