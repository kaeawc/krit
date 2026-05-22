package dev.jasonpearson.krit.intellij

// Single source of truth for which files krit can render diagnostics on
// in the IDE. The annotator and inspection both filter against this so
// they don't drift, and KritProjectService uses the same shape when
// deciding which document edits should trigger a rescan.
object KritFileFilter {
    fun isSupported(fileName: String): Boolean {
        val lower = fileName.lowercase()
        return lower.endsWith(".kt") ||
            lower.endsWith(".kts") ||
            lower.endsWith(".java") ||
            lower.endsWith(".xml") ||
            lower.endsWith(".gradle") ||
            lower.endsWith(".gradle.kts")
    }
}
