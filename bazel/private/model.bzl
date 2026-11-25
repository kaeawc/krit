"""The krit model of a Bazel target: what krit's FIR checker and type oracle compile it with.

krit reads the same JSON schema as the model krit-gradle-plugin exports
(internal/gradlemodel in the krit repository), so a Bazel target becomes one
project with one source set.
"""

load("@rules_java//java/common:java_info.bzl", "JavaInfo")

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

def krit_model_json(label, classpath, generated_source_jars = [], kind = "main"):
    """Renders a one-project krit model.

    Args:
      label: Label of the target; it names the project.
      classpath: list of jar paths, relative to the directory krit runs in.
      generated_source_jars: list of .srcjar paths compiled as generated
        context and never reported on.
      kind: "main" or "test".

    Returns:
      The model file's JSON text.
    """
    return json.encode_indent({
        "schema": _SCHEMA,
        "projects": [{
            "path": label_name(label),
            "dir": ".",
            "sourceSets": [{
                "name": label.name,
                "kind": kind,
                "platform": "jvm",
                "variant": "",
                "sourceDirs": [],
                "generatedSourceDirs": [],
                "generatedSourceJars": generated_source_jars,
                "classpath": classpath,
                "generatedClasspath": [],
                "bootClasspath": [],
                "projectDeps": [],
                "jvmTarget": "",
            }],
        }],
    }, indent = "  ") + "\n"
