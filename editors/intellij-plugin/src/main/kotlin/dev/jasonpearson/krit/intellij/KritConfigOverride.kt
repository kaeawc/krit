package dev.jasonpearson.krit.intellij

import java.io.File

/**
 * The "Config file" setting (Settings → Tools → Krit): a krit config passed
 * to krit as `--config` instead of the krit.yml / .krit.yml it would find at
 * the project root. Kept free of IntelliJ classes so it is unit-testable.
 *
 * The krit daemon always uses the project root's config, so while an override
 * is set the plugin runs every scan and fix through the CLI with --no-daemon
 * and doesn't start a daemon.
 */
object KritConfigOverride {
    /**
     * The absolute, normalized path of [setting], or null when it is blank.
     * A relative path resolves against [basePath] (the project root), so one
     * application-wide value such as `config/krit.yml` works per project; it
     * is null when there is no base path to resolve against.
     */
    fun resolve(setting: String?, basePath: String?): String? {
        val raw = setting?.trim().orEmpty()
        if (raw.isEmpty()) return null
        val file = File(raw)
        val absolute = when {
            file.isAbsolute -> file
            basePath.isNullOrBlank() -> return null
            else -> File(basePath, raw)
        }
        return absolute.toPath().normalize().toString()
    }

    /** The extra krit arguments for a scan or fix under [override]. */
    fun cliArgs(override: String?): List<String> =
        if (override == null) emptyList() else listOf("--config", override, "--no-daemon")

    /**
     * krit's report that it couldn't load the config, from its stderr: the
     * `warning: config: ...` line (internal/cli/scan loadScanConfig), without
     * the "warning: " prefix; null when there is none. With an explicit
     * --config that failed to load, krit carries on with no config, so the
     * plugin treats the run as failed.
     */
    fun loadFailure(stderr: String): String? =
        stderr.lineSequence().map { it.trim() }.firstOrNull { it.startsWith(CONFIG_WARNING) }
            ?.removePrefix("warning: ")

    private const val CONFIG_WARNING = "warning: config:"

    /**
     * True when [path] (a system-independent path, as the VFS reports it) is
     * the override file, so editing it rescans like editing krit.yml.
     */
    fun isOverride(override: String?, path: String?): Boolean {
        if (override == null || path == null) return false
        return File(path).toPath().normalize() == File(override).toPath().normalize()
    }
}
