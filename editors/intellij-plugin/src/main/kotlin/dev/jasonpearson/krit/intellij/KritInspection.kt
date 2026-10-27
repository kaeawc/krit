package dev.jasonpearson.krit.intellij

import com.intellij.codeInspection.LocalInspectionTool
import com.intellij.codeInspection.LocalQuickFix
import com.intellij.codeInspection.ProblemDescriptor
import com.intellij.codeInspection.ProblemsHolder
import com.intellij.openapi.components.service
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiElementVisitor

// The per-language inspections (Krit, KritJava, KritXml). They are the
// master switches for scanning (see KritInspectionToggle) and report the
// findings of rules that have no per-rule inspection (KritRuleInspection) in
// the profile, which happens when krit's rule list couldn't be read or a
// rule is newer than the list. Rules with a per-rule inspection are reported
// by it instead.
class KritInspection : LocalInspectionTool() {
    override fun buildVisitor(holder: ProblemsHolder, isOnTheFly: Boolean): PsiElementVisitor {
        return object : PsiElementVisitor() {
            override fun visitElement(element: PsiElement) {
                if (element.parent != null) {
                    return
                }
                val file = element.containingFile ?: return
                val path = file.virtualFile?.path ?: return
                val project = file.project
                val service = project.service<KritProjectService>()
                val covered = HashMap<String, Boolean>()
                for (finding in service.findingsFor(path)) {
                    val hasRuleInspection = covered.getOrPut(finding.rule) {
                        KritProfileRules.hasRuleInspection(project, finding.rule)
                    }
                    if (hasRuleInspection) continue
                    val range = KritRanges.rangeFor(file, finding)
                    val target = file.findElementAt(range.startOffset) ?: file
                    holder.registerProblem(
                        target,
                        finding.displayMessage,
                        KritSeverity.problemHighlightType(finding),
                        *KritQuickFixes.forFinding(finding),
                    )
                }
            }
        }
    }

    override fun getDisplayName(): String = "Krit"

    override fun getGroupDisplayName(): String = "Krit"

    override fun getShortName(): String = "Krit"

    override fun isEnabledByDefault(): Boolean = true
}

object KritQuickFixes {
    // Same choice as KritIntentions.forFinding: the suggestions, or the
    // autofix when its level is within the configured maximum, plus Suppress.
    fun forFinding(
        finding: KritFinding,
        maxFixLevel: String = KritSettingsState.maxFixLevel(),
    ): Array<LocalQuickFix> {
        val suppress = KritSuppressQuickFix(finding.rule)
        val applicable = finding.suggestedFixes.filter { it.edits.isNotEmpty() }
        if (applicable.isNotEmpty()) {
            return (
                applicable.map { KritApplySuggestionQuickFix(finding.findingId, it) } + suppress
            ).toTypedArray()
        }
        if (!finding.fixable || !KritFixLabels.isWithin(finding.fixLevel, maxFixLevel)) {
            return arrayOf(suppress)
        }
        val level = KritFixLabels.levelToApply(finding.fixLevel, maxFixLevel)
        return arrayOf(KritApplyFixesQuickFix(level, finding.findingId), suppress)
    }
}

class KritApplyFixesQuickFix(
    private val fixLevel: String?,
    private val findingId: String,
) : LocalQuickFix {
    override fun getFamilyName(): String = KritFixLabels.applyFixTitle(fixLevel)

    override fun applyFix(project: Project, descriptor: ProblemDescriptor) {
        project.service<KritProjectService>().applyFix(KritFixLabels.normalizeFixLevel(fixLevel), findingId)
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
