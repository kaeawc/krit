package dev.jasonpearson.krit.intellij

import java.io.File
import java.io.IOException
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit

/** A krit release version (`0.3.0`, `0.3.0-rc1`). Free of IntelliJ classes. */
data class KritVersion(
    val major: Int,
    val minor: Int,
    val patch: Int,
    val preRelease: String? = null,
) : Comparable<KritVersion> {
    // A pre-release sorts before its release; pre-release tags compare as text.
    override fun compareTo(other: KritVersion): Int {
        compareValues(major, other.major).takeIf { it != 0 }?.let { return it }
        compareValues(minor, other.minor).takeIf { it != 0 }?.let { return it }
        compareValues(patch, other.patch).takeIf { it != 0 }?.let { return it }
        return when {
            preRelease == other.preRelease -> 0
            preRelease == null -> 1
            other.preRelease == null -> -1
            else -> preRelease.compareTo(other.preRelease)
        }
    }

    override fun toString(): String = "$major.$minor.$patch" + (preRelease?.let { "-$it" } ?: "")

    companion object {
        private val PATTERN = Regex("""(?<![\w.])v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.]+))?""")

        /**
         * The first version in [text]: a tag (`v0.3.0`), a bare version, or
         * `krit --version` output (`krit 0.3.0`). Null for a source build
         * (`krit dev`) or text without a version.
         */
        fun parse(text: String?): KritVersion? {
            val match = PATTERN.find(text ?: return null) ?: return null
            val (major, minor, patch, pre) = match.destructured
            return try {
                KritVersion(major.toInt(), minor.toInt(), patch.toInt(), pre.ifEmpty { null })
            } catch (_: NumberFormatException) {
                null
            }
        }
    }
}

/**
 * The krit versions this plugin works with. A binary outside the range gets a
 * warning, not a refusal; the download offers only releases inside it.
 */
object KritCompatibility {
    /** The first release with the flags the plugin passes (`--no-daemon`, `--finding-id`, `daemon`). */
    val MIN_INCLUSIVE = KritVersion(0, 3, 0)

    /** The next major version may change the CLI the plugin depends on. */
    val MAX_EXCLUSIVE = KritVersion(1, 0, 0)

    val rangeText: String get() = ">= $MIN_INCLUSIVE and < $MAX_EXCLUSIVE"

    fun isCompatible(version: KritVersion): Boolean = version >= MIN_INCLUSIVE && version < MAX_EXCLUSIVE

    /**
     * The warning for a binary at [path] reporting [version], or null when it
     * is in range or its version is unknown (a source build prints `dev`).
     */
    fun warningFor(version: KritVersion?, path: String): String? {
        if (version == null || isCompatible(version)) return null
        val advice = if (version < MIN_INCLUSIVE) "Update krit" else "Update the Krit plugin"
        return "krit $version at $path is outside the versions this plugin supports ($rangeText). " +
            "Some features may not work. $advice, or choose another binary in Settings → Tools → Krit."
    }
}

/**
 * The version a krit binary reports for `krit --version`, cached per binary
 * (path, size, modification time) so it runs once per binary.
 */
class KritBinaryVersions(
    private val run: (File) -> String? = ::runVersion,
) {
    private val cache = ConcurrentHashMap<KritRuleCatalog.Key, Result>()

    private data class Result(val version: KritVersion?)

    /** [binary]'s version, or null when it is unknown (a source build, or the run failed). */
    fun versionOf(binary: File): KritVersion? =
        cache.computeIfAbsent(KritRuleCatalog.Key.of(binary)) { Result(KritVersion.parse(run(binary))) }.version

    companion object {
        private const val VERSION_TIMEOUT_SECONDS = 10L

        /** Runs `krit --version` and returns its output, or null when it fails. */
        fun runVersion(binary: File): String? {
            val process = try {
                ProcessBuilder(binary.absolutePath, "--version")
                    .redirectErrorStream(true)
                    .start()
            } catch (_: IOException) {
                return null
            }
            return try {
                if (!process.waitFor(VERSION_TIMEOUT_SECONDS, TimeUnit.SECONDS)) {
                    process.destroyForcibly()
                    return null
                }
                // The output is one short line, so it fits the pipe buffer.
                val out = process.inputStream.bufferedReader().readText()
                if (process.exitValue() != 0) null else out
            } catch (_: IOException) {
                null
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
                process.destroyForcibly()
                null
            }
        }
    }
}
