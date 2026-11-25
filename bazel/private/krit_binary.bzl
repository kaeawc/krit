"""Repository rules that download and verify a krit release's binaries and krit-fir jar."""

_RELEASES = "https://github.com/kaeawc/krit/releases/download/v{version}/"

_TOOLS = ["krit", "krit-lsp", "krit-mcp"]

_BUILD = """\
load("@bazel_skylib//rules:native_binary.bzl", "native_binary")

[native_binary(
    name = tool,
    src = "dist/" + tool + "{ext}",
    out = tool + "_bin{ext}",
    visibility = ["//visibility:public"],
) for tool in {tools}]
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

def _host(rctx):
    name = rctx.os.name.lower()
    arch = rctx.os.arch.lower()
    if name.startswith("mac os"):
        os = "darwin"
    elif name.startswith("windows"):
        os = "windows"
    elif name.startswith("linux"):
        os = "linux"
    else:
        fail("krit: no prebuilt binary for OS %r" % rctx.os.name)
    if arch in ("amd64", "x86_64", "x64"):
        arch = "amd64"
    elif arch in ("aarch64", "arm64"):
        arch = "arm64"
    else:
        fail("krit: no prebuilt binary for architecture %r" % rctx.os.arch)
    if os == "windows" and arch != "amd64":
        fail("krit: Windows releases are amd64 only")
    return os, arch

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
    os, arch = _host(rctx)
    ext = ".exe" if os == "windows" else ""
    local = _getenv(rctx, _LOCAL_DIST)
    tools = _TOOLS
    if local:
        # krit is required; the LSP and MCP servers only when they were built.
        tools = [t for t in _TOOLS if t == "krit" or rctx.path(local + "/" + t + ext).exists]
        for tool in tools:
            rctx.symlink(_local_file(rctx, local, tool + ext), "dist/" + tool + ext)
    else:
        version = _version(rctx)
        archive = "krit_%s_%s_%s.%s" % (version, os, arch, "zip" if os == "windows" else "tar.gz")
        base = _RELEASES.format(version = version)

        # Extract below dist/ so the files do not share a label with the
        # native_binary targets named after them.
        rctx.download_and_extract(url = base + archive, sha256 = _checksum(rctx, base, archive), output = "dist")
    rctx.file("BUILD.bazel", _BUILD.format(ext = ext, tools = repr(tools)))

krit_binary = repository_rule(
    implementation = _krit_binary_impl,
    attrs = {
        "version": attr.string(),
        "sha256": attr.string_dict(),
    },
    environ = ["KRIT_BAZEL_RELEASE", _LOCAL_DIST],
    doc = "Downloads the krit release archive for the host platform and exposes krit, krit-lsp and krit-mcp.",
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
