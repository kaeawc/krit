"""Repository rules that download and verify a krit release's binaries and krit-fir jar."""

_RELEASES = "https://github.com/kaeawc/krit/releases/download/v{version}/"

_TOOLS = ["krit", "krit-lsp", "krit-mcp"]

_BUILD = """\
load("{native_binary_bzl}", "native_binary")
load("{toolchain_bzl}", "krit_toolchain")

[native_binary(
    name = tool,
    src = "dist/" + tool + "{ext}",
    out = tool + "_bin{ext}",
    visibility = ["//visibility:public"],
) for tool in {tools}]

krit_toolchain(
    name = "toolchain",
    krit = ":krit",
    krit_lsp = {krit_lsp},
    krit_mcp = {krit_mcp},
    visibility = ["//visibility:public"],
)
"""

_FIR_BUILD = """\
exports_files(
    ["krit-fir.jar"],
    visibility = ["//visibility:public"],
)
"""

# KRIT_BAZEL_LOCAL_DIST names a directory with locally built krit binaries
# and krit-fir.jar (for example `go build -o DIR/krit ./cmd/krit` and
# `make fir-jar`). It replaces the release download, so an unreleased
# checkout can test bazel/ against the analyzer built from the same commit.
_LOCAL_DIST = "KRIT_BAZEL_LOCAL_DIST"

def _getenv(rctx, name):
    # rctx.getenv (Bazel 7.1+) re-fetches the repository when the variable
    # changes; `environ` below covers older releases.
    if hasattr(rctx, "getenv"):
        return rctx.getenv(name, "")
    return rctx.os.environ.get(name, "")

# The platforms krit releases binaries for: repository suffix -> (release
# OS, release architecture, @platforms constraints).
PLATFORMS = {
    "darwin_amd64": ("darwin", "amd64", ["@platforms//os:macos", "@platforms//cpu:x86_64"]),
    "darwin_arm64": ("darwin", "arm64", ["@platforms//os:macos", "@platforms//cpu:aarch64"]),
    "linux_amd64": ("linux", "amd64", ["@platforms//os:linux", "@platforms//cpu:x86_64"]),
    "linux_arm64": ("linux", "arm64", ["@platforms//os:linux", "@platforms//cpu:aarch64"]),
    "windows_amd64": ("windows", "amd64", ["@platforms//os:windows", "@platforms//cpu:x86_64"]),
}

def _host_platform(rctx):
    """Returns the PLATFORMS key of the machine running Bazel, or None."""
    name = rctx.os.name.lower()
    arch = rctx.os.arch.lower()
    if name.startswith("mac os"):
        os = "darwin"
    elif name.startswith("windows"):
        os = "windows"
    elif name.startswith("linux"):
        os = "linux"
    else:
        return None
    if arch in ("amd64", "x86_64", "x64"):
        arch = "amd64"
    elif arch in ("aarch64", "arm64"):
        arch = "arm64"
    else:
        return None
    key = os + "_" + arch
    return key if key in PLATFORMS else None

def _checksum(rctx, base, asset):
    pinned = rctx.attr.sha256.get(asset, "")
    if pinned:
        return pinned
    rctx.download(url = base + "checksums.txt", output = "checksums.txt")
    for line in rctx.read("checksums.txt").splitlines():
        parts = [p for p in line.split(" ") if p]
        if len(parts) == 2 and parts[1] == asset:
            return parts[0]
    fail("krit: %s is not listed in the v%s checksums.txt" % (asset, rctx.attr.version))

def _version(rctx):
    version = rctx.attr.version
    if not version or version == "0.0.0":
        # An unreleased checkout (local_path_override, git_override) has no
        # matching binaries; KRIT_BAZEL_RELEASE names a published one.
        version = _getenv(rctx, "KRIT_BAZEL_RELEASE")
    if not version:
        fail("krit: this krit module is unreleased; pin one with krit.toolchain(version = \"x.y.z\"), set KRIT_BAZEL_RELEASE, or point %s at a local build" % _LOCAL_DIST)
    return version

def _local_file(rctx, dist, name):
    path = rctx.path(dist + "/" + name)
    if not path.exists:
        fail("krit: %s=%s has no %s" % (_LOCAL_DIST, dist, name))

    # Re-fetch when the local build changes, so actions see the new file.
    rctx.watch(path)
    return path

def _krit_binary_impl(rctx):
    if rctx.attr.local:
        local = _getenv(rctx, _LOCAL_DIST)
        if not local:
            # Nothing refers to this repository without a local build, but
            # `bazel fetch --all` and `bazel vendor` fetch it anyway.
            rctx.file("BUILD.bazel", "# Unused: %s is not set.\n" % _LOCAL_DIST)
            return

        # A local build is for the host; .exe only matters on Windows.
        ext = ".exe" if rctx.os.name.lower().startswith("windows") else ""

        # krit is required; the LSP and MCP servers only when they were built.
        tools = [t for t in _TOOLS if t == "krit" or rctx.path(local + "/" + t + ext).exists]
        for tool in tools:
            rctx.symlink(_local_file(rctx, local, tool + ext), "dist/" + tool + ext)
    else:
        os, arch = rctx.attr.os, rctx.attr.arch
        ext = ".exe" if os == "windows" else ""
        tools = _TOOLS
        version = _version(rctx)
        archive = "krit_%s_%s_%s.%s" % (version, os, arch, "zip" if os == "windows" else "tar.gz")
        base = _RELEASES.format(version = version)

        # Extract below dist/ so the files do not share a label with the
        # native_binary targets named after them.
        rctx.download_and_extract(url = base + archive, sha256 = _checksum(rctx, base, archive), output = "dist")
    rctx.file("BUILD.bazel", _BUILD.format(
        native_binary_bzl = str(Label("@bazel_skylib//rules:native_binary.bzl")),
        toolchain_bzl = str(Label("//private:toolchain.bzl")),
        ext = ext,
        tools = repr(tools),
        krit_lsp = repr(":krit-lsp") if "krit-lsp" in tools else "None",
        krit_mcp = repr(":krit-mcp") if "krit-mcp" in tools else "None",
    ))

krit_binary = repository_rule(
    implementation = _krit_binary_impl,
    attrs = {
        "version": attr.string(),
        "sha256": attr.string_dict(),
        "os": attr.string(doc = "Release OS: darwin, linux or windows."),
        "arch": attr.string(doc = "Release architecture: amd64 or arm64."),
        "local": attr.bool(doc = "Use the host binaries in $KRIT_BAZEL_LOCAL_DIST instead of a release."),
    },
    environ = ["KRIT_BAZEL_RELEASE", _LOCAL_DIST],
    doc = "Downloads one platform's krit release archive and declares its krit toolchain (krit, krit-lsp, krit-mcp).",
)

_TOOLCHAIN = """\
toolchain(
    name = {name},
    exec_compatible_with = {constraints},
    toolchain = {impl},
    toolchain_type = {toolchain_type},
)
"""

_TARGET_TOOLS = """\
# krit, krit-lsp and krit-mcp for the target platform: `bazel run @krit//:krit`
# and krit_test, which run them on the machine the target is for (the host's
# binary when the target platform is none of the release platforms). Only
# the selected platform's archive is fetched.
[alias(
    name = tool,
    actual = {actual},
    visibility = ["//visibility:public"],
) for tool in {tools}]
"""

_CONFIG_SETTING = """\
config_setting(
    name = {name},
    constraint_values = {constraints},
)
"""

def _constraints(suffix):
    return repr([str(Label(c)) for c in PLATFORMS[suffix][2]])

def _krit_toolchains_impl(rctx):
    toolchain_type = repr(str(Label("//toolchains:toolchain_type")))
    tools = repr(["krit", "krit-lsp", "krit-mcp"])
    if _getenv(rctx, _LOCAL_DIST):
        # A local build runs on this machine only, so it is the one toolchain.
        host = _host_platform(rctx)
        build = _TOOLCHAIN.format(
            name = repr("local"),
            constraints = _constraints(host) if host else "[]",
            impl = repr("@%s//:toolchain" % rctx.attr.local_repo),
            toolchain_type = toolchain_type,
        ) + "\n" + _TARGET_TOOLS.format(
            actual = '"@%s//:" + tool' % rctx.attr.local_repo,
            tools = tools,
        )
    else:
        platforms = sorted(rctx.attr.platform_repos.items())
        build = "\n".join([
            _TOOLCHAIN.format(
                name = repr(suffix),
                constraints = _constraints(suffix),
                impl = repr("@%s//:toolchain" % repo),
                toolchain_type = toolchain_type,
            ) + "\n" + _CONFIG_SETTING.format(
                name = repr("is_" + suffix),
                constraints = _constraints(suffix),
            )
            for suffix, repo in platforms
        ])
        branches = ['":is_%s": "@%s//:" + tool' % (suffix, repo) for suffix, repo in platforms]

        # A target platform that names none of the release platforms (an
        # Android platform, one without @platforms constraints) gets the
        # host's binary, which is where such a test or `bazel run` runs.
        host = _host_platform(rctx)
        if host in rctx.attr.platform_repos:
            branches.append('"//conditions:default": "@%s//:" + tool' % rctx.attr.platform_repos[host])
            no_match = ""
        else:
            no_match = ", no_match_error = %r" % (
                "krit: no release binary for the target platform or the host (%s)" % ", ".join([s for s, _ in platforms])
            )
        build += "\n" + _TARGET_TOOLS.format(
            actual = "select({%s}%s)" % (", ".join(branches), no_match),
            tools = tools,
        )
    rctx.file("BUILD.bazel", build)

# Declares the toolchains without fetching any archive: Bazel fetches only
# the repository of the toolchain it resolves for an execution platform.
krit_toolchains = repository_rule(
    implementation = _krit_toolchains_impl,
    attrs = {
        "platform_repos": attr.string_dict(doc = "PLATFORMS key -> name of its krit_binary repository."),
        "local_repo": attr.string(doc = "Name of the krit_binary repository with local = True."),
    },
    environ = [_LOCAL_DIST],
    doc = "Declares a krit toolchain for each platform (or only the local build with KRIT_BAZEL_LOCAL_DIST) and target-platform aliases of the binaries.",
)

def _krit_fir_jar_impl(rctx):
    local = _getenv(rctx, _LOCAL_DIST)
    if local:
        rctx.symlink(_local_file(rctx, local, "krit-fir.jar"), "krit-fir.jar")
    else:
        version = _version(rctx)
        asset = "krit-fir-v%s.jar" % version
        base = _RELEASES.format(version = version)
        rctx.download(url = base + asset, sha256 = _checksum(rctx, base, asset), output = "krit-fir.jar")
    rctx.file("BUILD.bazel", _FIR_BUILD)

# A repository of its own: Bazel fetches it only when a type-aware target
# needs the jar, so Go-only users never download it.
krit_fir_jar = repository_rule(
    implementation = _krit_fir_jar_impl,
    attrs = {
        "version": attr.string(),
        "sha256": attr.string_dict(),
    },
    environ = ["KRIT_BAZEL_RELEASE", _LOCAL_DIST],
    doc = "Downloads the krit-fir jar of a krit release (the FIR checker and type oracle krit runs on the JVM).",
)
