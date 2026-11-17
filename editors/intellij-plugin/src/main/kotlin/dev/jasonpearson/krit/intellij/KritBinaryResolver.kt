package dev.jasonpearson.krit.intellij

import com.intellij.openapi.application.PathManager
import com.intellij.openapi.project.Project
import java.io.File

object KritBinaryResolver {
    /** Where the resolver found the binary, or why it found none. */
    sealed class Resolution {
        data class Found(val binary: File, val source: Source) : Resolution()

        /** A configured path that isn't executable; it doesn't fall through to PATH or a download. */
        data class ConfiguredMissing(val path: String, val source: Source) : Resolution()

        /** Nothing configured, nothing on PATH, nothing downloaded: the case that offers a download. */
        object NotFound : Resolution()

        /** The binary when one was found. */
        val fileOrNull: File? get() = (this as? Found)?.binary
    }

    enum class Source { PROJECT_SETTINGS, SETTINGS, SYSTEM_PROPERTY, ENVIRONMENT, PATH, DOWNLOADED }

    // Order of precedence:
    //   1. Settings → Tools → Krit → Project "Krit binary" (this project only)
    //   2. Settings → Tools → Krit "Krit binary" (application-wide)
    //   3. -Dkrit.binary
    //   4. KRIT_BINARY env var
    //   5. `krit` on PATH
    //   6. A krit the plugin downloaded from GitHub Releases (KritBinaryDownloads)
    //
    // A blank value at any level means unset and falls through to the next.
    // A configured override that doesn't resolve (path missing, not
    // executable) is ConfiguredMissing rather than falling through to PATH —
    // honoring the explicit choice the user made. This keeps debugging
    // legible: if you set a path, the resolver respects it exactly.
    fun resolve(
        env: Map<String, String> = System.getenv(),
        props: () -> String? = { System.getProperty("krit.binary") },
        settings: () -> String? = ::settingsBinaryPath,
        projectSettings: () -> String? = { null },
        downloaded: () -> File? = KritBinaryDownloads::installedBinary,
        windows: Boolean = KritPlatform.current()?.isWindows == true,
    ): Resolution {
        val configured = listOf(
            Source.PROJECT_SETTINGS to projectSettings,
            Source.SETTINGS to settings,
            Source.SYSTEM_PROPERTY to props,
            Source.ENVIRONMENT to { env["KRIT_BINARY"] },
        ).firstNotNullOfOrNull { (source, value) -> value()?.ifBlank { null }?.let { source to it } }
        if (configured != null) {
            val (source, path) = configured
            val f = File(path)
            return if (f.canExecute()) Resolution.Found(f, source) else Resolution.ConfiguredMissing(path, source)
        }
        val names = if (windows) listOf("krit.exe", "krit") else listOf("krit")
        for (dir in env["PATH"].orEmpty().split(File.pathSeparator)) {
            if (dir.isBlank()) continue
            for (name in names) {
                val f = File(dir, name)
                if (f.isFile && f.canExecute()) return Resolution.Found(f, Source.PATH)
            }
        }
        return downloaded()?.let { Resolution.Found(it, Source.DOWNLOADED) } ?: Resolution.NotFound
    }

    fun find(
        env: Map<String, String> = System.getenv(),
        props: () -> String? = { System.getProperty("krit.binary") },
        settings: () -> String? = ::settingsBinaryPath,
        projectSettings: () -> String? = { null },
        downloaded: () -> File? = KritBinaryDownloads::installedBinary,
    ): File? = resolve(env, props, settings, projectSettings, downloaded).fileOrNull

    // The binary for project: its own override first, then the global order.
    fun resolve(project: Project?): Resolution =
        resolve(projectSettings = { KritProjectSettingsState.binaryPathFor(project) })

    fun find(project: Project?): File? = resolve(project).fileOrNull

    private fun settingsBinaryPath(): String? =
        KritSettingsState.getSafely()?.state?.binaryPath?.takeIf { it.isNotBlank() }
}

/** Where downloaded krit binaries live: `<IDE system dir>/krit/bin/<version>/krit`. */
object KritBinaryDownloads {
    fun installRoot(): File = File(PathManager.getSystemPath(), "krit/bin")

    /** The newest downloaded krit, or null when none is (or the IDE paths aren't available). */
    fun installedBinary(): File? = try {
        KritBinaryInstaller.installedBinary(installRoot())
    } catch (_: RuntimeException) {
        null
    } catch (_: LinkageError) {
        null
    }
}
