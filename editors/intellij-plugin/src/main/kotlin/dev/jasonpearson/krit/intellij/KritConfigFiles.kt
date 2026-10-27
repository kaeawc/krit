package dev.jasonpearson.krit.intellij

/**
 * The project-root krit config files the plugin watches. Mirrors
 * `config.Filenames` in internal/config/config.go, which the CLI probes (in
 * this order) when no --config is given. Kept free of IntelliJ classes so the
 * matching is unit-testable without the platform.
 */
object KritConfigFiles {
    val FILENAMES: List<String> = listOf("krit.yml", ".krit.yml")

    /**
     * True when [path] names one of [FILENAMES] directly in [basePath]. Both
     * are system-independent ('/'-separated) paths, as the VFS reports them.
     */
    fun isProjectConfig(basePath: String?, path: String): Boolean {
        if (basePath.isNullOrBlank()) return false
        val base = basePath.trimEnd('/')
        val slash = path.lastIndexOf('/')
        if (slash < 0) return false
        return path.substring(0, slash) == base && path.substring(slash + 1) in FILENAMES
    }

    /** True when any of [paths] is a project config file (see [isProjectConfig]). */
    fun anyProjectConfig(basePath: String?, paths: Iterable<String?>): Boolean =
        paths.any { it != null && isProjectConfig(basePath, it) }
}
