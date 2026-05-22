package dev.jasonpearson.krit.intellij

import java.io.File

object KritBinaryResolver {
    // Order of precedence:
    //   1. -Dkrit.binary
    //   2. KRIT_BINARY env var
    //   3. `krit` on PATH
    fun find(env: Map<String, String> = System.getenv(), props: () -> String? = { System.getProperty("krit.binary") }): File? {
        val configured = props() ?: env["KRIT_BINARY"]
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
}
