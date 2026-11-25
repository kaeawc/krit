"""The Kotlin compiler settings of a rules_kotlin target, for krit's model.

A rules_kotlin compile action uses the toolchain's language, API and JVM
target versions, the target's `kotlinc_opts` (or else the toolchain's
`kotlinc_options`), and the compiler plugins of its `plugins` attribute and
of its deps' `exported_compiler_plugins`. This reads the same values, so
krit's FIR checker and type oracle compile the target the way the build
does. krit then filters the flags through the allowlist it applies to the
Gradle model (internal/gradlemodel/kotlin_settings.go): both paths reject
the same arguments.
"""

# rules_kotlin exports its providers only from kotlin/internal; IDE aspects
# (bazel-bsp, the IntelliJ plugin) load them from there too.
# buildifier: disable=bzl-visibility
load(
    "@rules_kotlin//kotlin/internal:defs.bzl",
    "KtCompilerPluginInfo",
    "KtJvmInfo",
    "KtPluginConfiguration",
    _KT_TOOLCHAIN_TYPE = "TOOLCHAIN_TYPE",
)

# buildifier: disable=bzl-visibility
load("@rules_kotlin//kotlin/internal:opts.bzl", "KotlincOptions", "kotlinc_options_to_flags")

# Optional: Java-only builds have no Kotlin toolchain to resolve.
KT_TOOLCHAIN = config_common.toolchain_type(_KT_TOOLCHAIN_TYPE, mandatory = False)

def _attr(rule_attr, name):
    return getattr(rule_attr, name, None) or []

# kt_compiler_plugin option templates that the KotlinCompile action expands
# to its own directories; krit compiles for analysis only and writes nothing.
_TEMPLATES = ["{generatedClasses}", "{generatedSources}", "{temp}", "{classpath}"]

def _is_template(value):
    for t in _TEMPLATES:
        if t in value:
            return True
    return False

def _plugins(rule_attr):
    """The compiler plugins rules_kotlin applies to a target, as structs."""
    targets = list(_attr(rule_attr, "plugins"))
    for dep in _attr(rule_attr, "deps"):
        if KtJvmInfo in dep and dep[KtJvmInfo]:
            exported = getattr(dep[KtJvmInfo], "exported_compiler_plugins", None)
            if exported:
                targets.extend(exported.to_list())
    plugins = {}
    for t in targets:
        if KtCompilerPluginInfo in t:
            info = t[KtCompilerPluginInfo]

            # Stubs-only plugins run for kapt, not for the real compilation.
            if getattr(info, "compile", True):
                plugins[info.id] = info
    configs = {}
    for t in targets:
        if KtPluginConfiguration in t:
            cfg = t[KtPluginConfiguration]
            configs.setdefault(cfg.id, []).append(cfg)

    out = []
    for plugin_id, info in plugins.items():
        classpath = [info.classpath]
        options = list(info.options)
        if plugin_id in configs and hasattr(info, "merge_cfgs"):
            merged = info.merge_cfgs(info, configs[plugin_id])
            classpath.append(merged.classpath)
            options.extend(merged.options)
        out.append(struct(
            id = plugin_id,
            classpath = depset(transitive = classpath),
            options = [
                "plugin:%s:%s=%s" % (plugin_id, o.key, o.value)
                for o in options
                if not _is_template(o.value)
            ],
        ))
    return out

def kotlin_settings(ctx, target):
    """Returns the target's Kotlin compiler settings.

    Args:
      ctx: the aspect or rule context; its toolchains include KT_TOOLCHAIN.
      target: the analyzed target.

    Returns:
      struct(kotlin, jvm_target, plugin_jars, generated_source_jars):
      kotlin is the model's `kotlin` dict without `compilerPluginClasspath`
      (None for a target rules_kotlin does not compile), plugin_jars a
      depset of the plugin jars (the caller renders their paths into
      `compilerPluginClasspath` and adds them to its inputs),
      generated_source_jars a list of KSP and kapt source jar outputs.
    """
    none = struct(kotlin = None, jvm_target = "", plugin_jars = depset(), generated_source_jars = [])
    if KtJvmInfo not in target:
        return none
    toolchain = ctx.toolchains[_KT_TOOLCHAIN_TYPE]
    if toolchain == None:
        return none
    rule_attr = ctx.rule.attr
    opts_target = getattr(rule_attr, "kotlinc_opts", None)
    opts = opts_target[KotlincOptions] if opts_target else getattr(toolchain, "kotlinc_options", None)
    flags = kotlinc_options_to_flags(opts) if opts else []
    language_version = (getattr(opts, "language_version", None) if opts else None) or toolchain.language_version
    api_version = (getattr(opts, "api_version", None) if opts else None) or toolchain.api_version
    jvm_target = (getattr(opts, "jvm_target", None) if opts else None) or toolchain.jvm_target

    plugins = _plugins(rule_attr)
    plugin_jars = depset(transitive = [p.classpath for p in plugins])
    options = []
    for p in plugins:
        options.extend(p.options)

    generated = getattr(target[KtJvmInfo], "additional_generated_source_jars", None) or []
    return struct(
        kotlin = {
            "languageVersion": language_version or "",
            "apiVersion": api_version or "",
            "optIn": [],
            "freeCompilerArgs": flags,
            "progressiveMode": False,
            "pluginVersion": "",
            "compilerPluginOptions": options,
        },
        jvm_target = jvm_target or "",
        plugin_jars = plugin_jars,
        generated_source_jars = list(generated),
    )
