"""The Android inputs of a rules_android or rules_kotlin Android target, for krit.

For `android_library`, `kt_android_library`, `android_binary` and the like
this supplies what AGP gives krit under Gradle:

- `android.jar` from the Android SDK toolchain (`AndroidSdkInfo.android_jar`)
  as the boot classpath, instead of a guess from `$ANDROID_HOME`;
- the R class jars (`AndroidLibraryResourceClassJarProvider`, and the
  target's own `AndroidIdeInfo.resource_jar`), so `R.string.x` resolves;
- the target's own manifest and resource files (its `manifest` and
  `resource_files` attributes), so krit's manifest and resource rules run.
"""

load(
    "@rules_android//providers:providers.bzl",
    "AndroidIdeInfo",
    "AndroidLibraryResourceClassJarProvider",
    "StarlarkAndroidResourcesInfo",
)

_ANDROID_SDK_TOOLCHAIN_TYPE = "@rules_android//toolchains/android_sdk:toolchain_type"

KritAndroidSdkInfo = provider(
    doc = "The Android SDK an Android target compiles against.",
    fields = {"android_jar": "File: the SDK's android.jar, or None"},
)

def _krit_android_sdk_aspect_impl(_target, ctx):
    sdk = getattr(ctx.toolchains[_ANDROID_SDK_TOOLCHAIN_TYPE], "android_sdk_info", None)
    return [KritAndroidSdkInfo(android_jar = sdk.android_jar if sdk else None)]

# Resolves the Android SDK toolchain for Android targets only (those that
# advertise AndroidIdeInfo: android_library, kt_android_library,
# android_binary, ...). The krit aspects require it instead of declaring the
# toolchain themselves: without an Android SDK, rules_android's placeholder
# toolchain fails resolution even for an optional toolchain, which would
# break JVM-only builds.
krit_android_sdk_aspect = aspect(
    implementation = _krit_android_sdk_aspect_impl,
    required_providers = [AndroidIdeInfo],
    toolchains = [_ANDROID_SDK_TOOLCHAIN_TYPE],
    doc = "Provides KritAndroidSdkInfo (android.jar) for Android targets.",
)

def _is_android(target):
    return (AndroidIdeInfo in target or
            StarlarkAndroidResourcesInfo in target or
            AndroidLibraryResourceClassJarProvider in target)

def _res_dir(path):
    # Android resources are <res>/<type>[-<qualifiers>]/<file>.
    return path.rpartition("/")[0].rpartition("/")[0]

def android_inputs(ctx, target):
    """Returns the Android inputs krit needs for a target.

    Args:
      ctx: aspect context.
      target: the analyzed target; for an Android target the aspect
        requires krit_android_sdk_aspect, which provides its android.jar.

    Returns:
      struct(is_android, boot_classpath, r_jars, manifest, resource_files):
      boot_classpath is a list (android.jar), r_jars a depset of R class
      jars, manifest the target's source manifest File or None, and
      resource_files a list of the target's own resource Files.
    """
    if not _is_android(target):
        return struct(is_android = False, boot_classpath = [], r_jars = depset(), manifest = None, resource_files = [])
    boot = []
    if KritAndroidSdkInfo in target and target[KritAndroidSdkInfo].android_jar:
        boot.append(target[KritAndroidSdkInfo].android_jar)

    r_jars = []
    if AndroidLibraryResourceClassJarProvider in target:
        r_jars.append(target[AndroidLibraryResourceClassJarProvider].jars)
    own = []
    if AndroidIdeInfo in target:
        resource_jar = target[AndroidIdeInfo].resource_jar
        if resource_jar and resource_jar.class_jar:
            own.append(resource_jar.class_jar)

    rule_files = ctx.rule.file
    manifest = getattr(rule_files, "manifest", None)
    resource_files = [
        f
        for f in getattr(ctx.rule.files, "resource_files", [])
        if f.is_source and _res_dir(f.path)
    ]
    return struct(
        is_android = True,
        boot_classpath = boot,
        r_jars = depset(own, transitive = r_jars),
        manifest = manifest if manifest and manifest.is_source else None,
        resource_files = resource_files,
    )

def resource_dirs_build_file(build_path, resource_files, path):
    """A BUILD file naming a target's resource directories, for krit.

    krit finds a Bazel target's res/ directories in the resource_files globs
    of the BUILD files it scans. An action sees the target's resource files
    but not its BUILD file, so it scans this one instead; the globs are
    relative to it.

    Args:
      build_path: path of the generated BUILD file, relative to the same
        root as the resource files (both exec paths or both runfiles paths).
      resource_files: list of resource Files.
      path: function from File to its path relative to that root.

    Returns:
      The BUILD file's text, or None for no resources.
    """
    dirs = {}
    for f in resource_files:
        dirs[_res_dir(path(f))] = True
    if not dirs:
        return None
    up = "../" * len(build_path.rpartition("/")[0].split("/"))
    globs = ['"%s%s/**"' % (up, d) for d in sorted(dirs)]
    return "android_library(\n    name = \"resources\",\n    resource_files = glob([%s]),\n)\n" % ", ".join(globs)

def sdk_level_args(min_sdk, target_sdk, manifest_values = {}):
    """krit's --min-sdk / --target-sdk for one action.

    They are the levels of sources whose own manifest declares none; krit
    still reads a manifest's <uses-sdk> where there is one. Bazel builds
    often set the levels only on the android_binary (manifest_values) or
    build-wide, where krit cannot see them.

    Args:
      min_sdk: int, --@krit//:min_sdk_version (0: unset).
      target_sdk: int, --@krit//:target_sdk_version (0: unset).
      manifest_values: the analyzed android_binary's manifest_values; a
        minSdkVersion or targetSdkVersion that is a number wins over the
        flag. Values with make variables are left to the flag.

    Returns:
      list of krit arguments.
    """
    args = []
    for flag, level, key in [
        ("--min-sdk", min_sdk, "minSdkVersion"),
        ("--target-sdk", target_sdk, "targetSdkVersion"),
    ]:
        value = manifest_values.get(key, "")
        if value.isdigit():
            level = int(value)
        if level > 0:
            args += [flag, str(level)]
    return args
