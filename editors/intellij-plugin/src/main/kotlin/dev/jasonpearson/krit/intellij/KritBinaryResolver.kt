package dev.jasonpearson.krit.intellij

import java.io.File

object KritBinaryResolver {
    // Order of precedence:
    //   1. Settings → Tools → Krit "Krit binary" (when set and executable)
    //   2. -Dkrit.binary
    //   3. KRIT_BINARY env var
    //   4. `krit` on PATH
    //
    // A configured override that doesn't resolve (path missing, not
    // executable) returns null rather than falling through to PATH —
    // honoring the explicit choice the user made. This keeps debugging
    // legible: if you set a path, the resolver respects it exactly.
    fun find(
        env: Map<String, String> = System.getenv(),
        props: () -> String? = { System.getProperty("krit.binary") },
        settings: () -> String? = ::settingsBinaryPath,
    ): File? {
        val configured = settings()?.ifBlank { null }
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

    private fun settingsBinaryPath(): String? =
        KritSettingsState.getSafely()?.state?.binaryPath?.takeIf { it.isNotBlank() }
}
