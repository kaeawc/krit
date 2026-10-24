package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.daemon.HighlightDisplayKey
import com.intellij.openapi.progress.ProcessCanceledException
import com.intellij.openapi.project.Project
import com.intellij.profile.codeInspection.InspectionProjectProfileManager

// Krit registers one inspection per language in plugin.xml. Scans only feed
// those inspections, so they are worth running while at least one of them is
// enabled in the project's active inspection profile.
object KritInspectionToggle {
    val SHORT_NAMES = listOf("Krit", "KritJava", "KritXml")

    fun anyEnabled(isEnabled: (shortName: String) -> Boolean): Boolean = SHORT_NAMES.any(isEnabled)

    // True when any Krit inspection is enabled in the current profile. An
    // unknown key or an unreadable profile counts as enabled, so a lookup
    // failure never silently stops scans.
    fun anyEnabled(project: Project): Boolean = try {
        val profile = InspectionProjectProfileManager.getInstance(project).currentProfile
        anyEnabled { name ->
            val key = HighlightDisplayKey.find(name)
            key == null || profile.isToolEnabled(key)
        }
    } catch (e: ProcessCanceledException) {
        throw e
    } catch (_: RuntimeException) {
        true
    }
}
