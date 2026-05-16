package dev.jasonpearson.krit.intellij

import com.intellij.codeInspection.LocalInspectionTool
import com.intellij.codeInspection.LocalQuickFix
import com.intellij.codeInspection.ProblemDescriptor
import com.intellij.codeInspection.ProblemHighlightType
import com.intellij.codeInspection.ProblemsHolder
import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiElementVisitor

class KritInspection : LocalInspectionTool() {
    override fun buildVisitor(holder: ProblemsHolder, isOnTheFly: Boolean): PsiElementVisitor {
        return object : PsiElementVisitor() {
            override fun visitElement(element: PsiElement) {
                if (element.parent != null) {
                    return
                }
                val file = element.containingFile ?: return
                val path = file.virtualFile?.path ?: return
                val service = file.project.service<KritProjectService>()
                for (finding in service.findingsFor(path)) {
                    val range = KritRanges.rangeFor(file, finding)
                    val target = file.findElementAt(range.startOffset) ?: file
                    holder.registerProblem(
                        target,
                        finding.displayMessage,
                        problemHighlightType(finding),
                        *quickFixes(finding),
                    )
                }
            }
        }
    }

    override fun getDisplayName(): String = "Krit"

    override fun getGroupDisplayName(): String = "Krit"

    override fun getShortName(): String = "Krit"

    override fun isEnabledByDefault(): Boolean = true

    private fun problemHighlightType(finding: KritFinding): ProblemHighlightType {
        return when (finding.severity.lowercase()) {
            "error" -> ProblemHighlightType.ERROR
            "info" -> ProblemHighlightType.INFORMATION
            else -> ProblemHighlightType.WARNING
        }
    }

    private fun quickFixes(finding: KritFinding): Array<LocalQuickFix> {
        val applicable = finding.suggestedFixes.filter { it.edits.isNotEmpty() }
        if (applicable.isNotEmpty()) {
            return applicable
                .map { KritApplySuggestionQuickFix(finding.findingId, it) }
                .toTypedArray()
        }
        if (!finding.fixable) {
            return emptyArray()
        }
        return arrayOf(KritApplyFixesQuickFix(finding.fixLevel))
    }
}

class KritApplyFixesQuickFix(private val fixLevel: String?) : LocalQuickFix {
    override fun getFamilyName(): String = KritFixLabels.applyFixesTitle(fixLevel)

    override fun applyFix(project: Project, descriptor: ProblemDescriptor) {
        project.service<KritProjectService>().applyFixes(KritFixLabels.normalizeFixLevel(fixLevel))
    }
}

class KritApplySuggestionQuickFix(
    private val findingId: String,
    private val suggestion: KritSuggestedFix,
) : LocalQuickFix {
    override fun getName(): String = KritFixLabels.suggestionTitle(suggestion)

    override fun getFamilyName(): String = KritFixLabels.SUGGESTION_FAMILY_NAME

    override fun applyFix(project: Project, descriptor: ProblemDescriptor) {
        project.service<KritProjectService>().applySuggestion(findingId, suggestion.id)
    }
}
