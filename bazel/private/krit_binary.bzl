"""Repository rule that downloads and verifies a krit release archive."""

_RELEASES = "https://github.com/kaeawc/krit/releases/download/v{version}/"

_BUILD = """\
load("@bazel_skylib//rules:native_binary.bzl", "native_binary")

[native_binary(
    name = tool,
    src = "dist/" + tool + "{ext}",
    out = tool + "_bin{ext}",
    visibility = ["//visibility:public"],
) for tool in ["krit", "krit-lsp", "krit-mcp"]]
"""

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

def _checksum(rctx, base, archive):
    pinned = rctx.attr.sha256.get(archive, "")
    if pinned:
        return pinned
    rctx.download(url = base + "checksums.txt", output = "checksums.txt")
    for line in rctx.read("checksums.txt").splitlines():
        parts = [p for p in line.split(" ") if p]
        if len(parts) == 2 and parts[1] == archive:
            return parts[0]
    fail("krit: %s is not listed in the v%s checksums.txt" % (archive, rctx.attr.version))

def _krit_binary_impl(rctx):
    version = rctx.attr.version
    if not version or version == "0.0.0":
        # An unreleased checkout (local_path_override, git_override) has no
        # matching binaries; KRIT_BAZEL_RELEASE names a published one.
        version = rctx.os.environ.get("KRIT_BAZEL_RELEASE", "")
    if not version:
        fail("krit: this krit module is unreleased; pin one with krit.toolchain(version = \"x.y.z\") or set KRIT_BAZEL_RELEASE")
    os, arch = _host(rctx)
    ext = ".exe" if os == "windows" else ""
    archive = "krit_%s_%s_%s.%s" % (version, os, arch, "zip" if os == "windows" else "tar.gz")
    base = _RELEASES.format(version = version)
    # Extract below dist/ so the files do not share a label with the
    # native_binary targets named after them.
    rctx.download_and_extract(url = base + archive, sha256 = _checksum(rctx, base, archive), output = "dist")
    rctx.file("BUILD.bazel", _BUILD.format(ext = ext))

krit_binary = repository_rule(
    implementation = _krit_binary_impl,
    attrs = {
        "version": attr.string(),
        "sha256": attr.string_dict(),
    },
    environ = ["KRIT_BAZEL_RELEASE"],
    doc = "Downloads the krit release archive for the host platform and exposes krit, krit-lsp and krit-mcp.",
)
