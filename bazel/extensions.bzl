"""Module extension that fetches the prebuilt krit release for the host.

By default the binary version is this module's version. The krit-fir jar
of the same release (the JVM FIR checker and type oracle that type-aware
targets run) is a separate repository, fetched only when one is built. A root module can
pin another release, and optionally its archive checksums:

    krit = use_extension("@krit//:extensions.bzl", "krit")
    krit.toolchain(
        version = "0.3.0",
        sha256 = {"krit_0.3.0_linux_amd64.tar.gz": "..."},
    )
"""

load("//private:krit_binary.bzl", "krit_binary", "krit_fir_jar")

_toolchain = tag_class(
    attrs = {
        "version": attr.string(
            doc = "krit release to download, without the leading v. Defaults to the krit module's version.",
        ),
        "sha256": attr.string_dict(
            doc = "Release asset name (an archive or krit-fir-vX.Y.Z.jar) to SHA-256. Without an entry for an asset, the release's checksums.txt is used.",
        ),
    },
)

def _krit_impl(module_ctx):
    version = ""
    sha256 = {}
    default_version = ""
    for mod in module_ctx.modules:
        if mod.name == "krit":
            default_version = mod.version
        for tag in mod.tags.toolchain:
            # The root module's tag wins; otherwise the first one seen.
            if mod.is_root or not version:
                version = tag.version or version
                sha256 = tag.sha256 or sha256
    krit_binary(
        name = "krit_bin",
        version = version or default_version,
        sha256 = sha256,
    )
    krit_fir_jar(
        name = "krit_fir",
        version = version or default_version,
        sha256 = sha256,
    )

krit = module_extension(
    implementation = _krit_impl,
    tag_classes = {"toolchain": _toolchain},
)
