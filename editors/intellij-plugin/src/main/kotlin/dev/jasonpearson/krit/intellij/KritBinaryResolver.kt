package dev.jasonpearson.krit.intellij

import com.intellij.openapi.project.Project
import java.io.File

object KritBinaryResolver {
    // Order of precedence:
    //   1. Settings → Tools → Krit → Project "Krit binary" (this project only)
    //   2. Settings → Tools → Krit "Krit binary" (application-wide)
    //   3. -Dkrit.binary
    //   4. KRIT_BINARY env var
    //   5. `krit` on PATH
    //
    // A blank value at any level means unset and falls through to the next.
    // A configured override that doesn't resolve (path missing, not
    // executable) returns null rather than falling through to PATH —
    // honoring the explicit choice the user made. This keeps debugging
    // legible: if you set a path, the resolver respects it exactly.
    fun find(
        env: Map<String, String> = System.getenv(),
        props: () -> String? = { System.getProperty("krit.binary") },
        settings: () -> String? = ::settingsBinaryPath,
        projectSettings: () -> String? = { null },
    ): File? {
        val configured = projectSettings()?.ifBlank { null }
            ?: settings()?.ifBlank { null }
            ?: props()?.ifBlank { null }
            ?: env["KRIT_BINARY"]?.ifBlank { null }
        if (configured != null) {
            val f = File(configured)
            return if (f.canExecute()) f else null
        }
        val pathEntry = env["PATH"] ?: return null
        for (dir in pathEntry.split(File.pathSeparator)) {
            if (dir.isBlank()) continue
            val f = File(dir, "krit")
            if (f.canExecute()) return f
        }
        return null
    }

    // The binary for project: its own override first, then the global order.
    fun find(project: Project?): File? =
        find(projectSettings = { KritProjectSettingsState.binaryPathFor(project) })

    private fun settingsBinaryPath(): String? =
        KritSettingsState.getSafely()?.state?.binaryPath?.takeIf { it.isNotBlank() }
}
