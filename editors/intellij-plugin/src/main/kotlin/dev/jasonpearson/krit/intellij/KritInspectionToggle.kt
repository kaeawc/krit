package dev.jasonpearson.krit.intellij

import com.intellij.openapi.progress.ProcessCanceledException
import com.intellij.openapi.project.Project
import com.intellij.profile.codeInspection.InspectionProjectProfileManager

// Krit registers one inspection per language in plugin.xml. Scans only feed
// those inspections, so they are worth running while at least one of them is
// enabled in the project's active inspection profile.
object KritInspectionToggle {
    val SHORT_NAMES = listOf("Krit", "KritJava", "KritXml")

    fun anyEnabled(isEnabled: (shortName: String) -> Boolean): Boolean = SHORT_NAMES.any(isEnabled)

    // True when any Krit inspection is enabled in the current profile, in its
    // default scope or any named scope (an inspection on only for, say, test
    // sources still needs scans). An unknown tool or an unreadable profile
    // counts as enabled, so a lookup failure never silently stops scans.
    fun anyEnabled(project: Project): Boolean = try {
        val profile = InspectionProjectProfileManager.getInstance(project).currentProfile
        anyEnabled { name ->
            val tools = profile.getToolsOrNull(name, project)
            tools == null || (tools.isEnabled && tools.tools.any { it.isEnabled })
        }
    } catch (e: ProcessCanceledException) {
        throw e
    } catch (_: RuntimeException) {
        true
    }
}
