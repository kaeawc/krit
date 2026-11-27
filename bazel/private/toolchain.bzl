"""The krit toolchain: krit, krit-lsp and krit-mcp for one execution platform.

The module registers one toolchain per platform krit releases binaries for
(see krit_binary.bzl), constrained on `exec_compatible_with`, so Bazel runs
the binary built for the machine that executes each action, local or
remote, and fetches only that platform's release archive.

A root module can register its own toolchain first, for example to run a
locally built krit:

    load("@krit//:defs.bzl", "krit_toolchain")

    krit_toolchain(
        name = "krit_local",
        krit = "@my_krit_build//:krit",
    )

    toolchain(
        name = "krit_local_toolchain",
        toolchain = ":krit_local",
        toolchain_type = "@krit//toolchains:toolchain_type",
    )

and in MODULE.bazel `register_toolchains("//:krit_local_toolchain")`.
"""

KRIT_TOOLCHAIN_TYPE = Label("//toolchains:toolchain_type")

_TOOLS = ["krit", "krit_lsp", "krit_mcp"]

def _krit_toolchain_impl(ctx):
    fields = {}
    for tool in _TOOLS:
        target = getattr(ctx.attr, tool)
        fields[tool] = target[DefaultInfo].files_to_run if target else None
        fields[tool + "_runfiles"] = target[DefaultInfo].default_runfiles if target else None
    return [platform_common.ToolchainInfo(**fields)]

krit_toolchain = rule(
    implementation = _krit_toolchain_impl,
    doc = """Declares the krit executables of one execution platform.

The ToolchainInfo has `krit`, `krit_lsp` and `krit_mcp` (FilesToRunProvider,
or None for a server the toolchain lacks) and `<tool>_runfiles`.
""",
    attrs = {
        "krit": attr.label(
            mandatory = True,
            executable = True,
            cfg = "exec",
            doc = "The krit executable.",
        ),
        "krit_lsp": attr.label(
            executable = True,
            cfg = "exec",
            doc = "The krit-lsp executable, if available.",
        ),
        "krit_mcp": attr.label(
            executable = True,
            cfg = "exec",
            doc = "The krit-mcp executable, if available.",
        ),
    },
)

def _krit_tool_impl(ctx):
    toolchain = ctx.toolchains[KRIT_TOOLCHAIN_TYPE]
    tool = getattr(toolchain, ctx.attr.tool)
    if tool == None:
        fail("krit: the resolved krit toolchain has no %s" % ctx.attr.tool.replace("_", "-"))
    exe = tool.executable
    out = ctx.actions.declare_file(ctx.label.name + (".exe" if exe.extension == "exe" else ""))
    ctx.actions.symlink(output = out, target_file = exe, is_executable = True)
    runfiles = ctx.runfiles(files = [out]).merge(getattr(toolchain, ctx.attr.tool + "_runfiles"))
    return [DefaultInfo(executable = out, files = depset([out]), runfiles = runfiles)]

# `bazel run @krit//:krit` and `data = ["@krit//:krit"]`: the binary of the
# krit toolchain resolved for this target's execution platform.
krit_tool = rule(
    implementation = _krit_tool_impl,
    executable = True,
    toolchains = [KRIT_TOOLCHAIN_TYPE],
    attrs = {
        "tool": attr.string(mandatory = True, values = _TOOLS),
    },
)
