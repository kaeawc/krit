package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.openapi.components.service
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiFile

class KritExternalAnnotator : ExternalAnnotator<PsiFile, List<KritFinding>>() {
    override fun collectInformation(file: PsiFile): PsiFile? {
        val name = file.virtualFile?.name ?: return null
        if (!KritFileFilter.isSupported(name)) {
            return null
        }
        return file
    }

    override fun doAnnotate(collectedInfo: PsiFile?): List<KritFinding> {
        val file = collectedInfo ?: return emptyList()
        val path = file.virtualFile?.path ?: return emptyList()
        val service = file.project.service<KritProjectService>()
        return service.findingsFor(path)
    }

    // Per-rule inspections turned off in the profile hide their findings
    // here too, and a level changed there replaces krit's severity (see
    // KritRuleFilter for the precedence with krit.yml).
    override fun apply(file: PsiFile, annotationResult: List<KritFinding>?, holder: AnnotationHolder) {
        val findings = annotationResult.orEmpty()
        if (findings.isEmpty()) return
        val profile = KritProfileRules.profileOf(file.project)
        val shown = KritRuleFilter.apply(findings) { rule ->
            profile?.let { KritProfileRules.settingFor(it, file, rule) }
        }
        for ((finding, level) in shown) {
            val annotation = holder.newAnnotation(KritSeverity.highlightSeverity(level), finding.displayMessage)
                .range(KritRanges.rangeFor(file, finding))
            for (intention in KritIntentions.forFinding(finding)) {
                annotation.withFix(intention)
            }
            annotation.create()
        }
    }
}

object KritIntentions {
    // Suggestions and the autofix slot are mutually exclusive per finding:
    // when a rule emits suggestions the user picks one explicitly, so the
    // catch-all "apply auto-fixes" entry would conflict. Suppress is
    // orthogonal and always offered.
    //
    // The autofix is offered only when its level is within Settings → Tools
    // → Krit → "Maximum fix level" (krit's own --fix-level default is
    // idiomatic). A fixable finding that doesn't say its level is fixed at
    // that maximum.
    fun forFinding(
        finding: KritFinding,
        maxFixLevel: String = KritSettingsState.maxFixLevel(),
    ): List<IntentionAction> {
        val suppress = KritSuppressIntention(finding.rule)
        val applicable = finding.suggestedFixes.filter { it.edits.isNotEmpty() }
        if (applicable.isNotEmpty()) {
            return applicable.map { KritApplySuggestionIntention(finding.findingId, it) } + suppress
        }
        if (finding.fixable && KritFixLabels.isWithin(finding.fixLevel, maxFixLevel)) {
            val level = KritFixLabels.levelToApply(finding.fixLevel, maxFixLevel)
            return listOf(KritApplyFixesIntention(level, finding.findingId), suppress)
        }
        return listOf(suppress)
    }
}

class KritApplyFixesIntention(
    private val fixLevel: String?,
    private val findingId: String,
) : IntentionAction {
    override fun getText(): String = KritFixLabels.applyFixTitle(fixLevel)

    override fun getFamilyName(): String = text

    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean = true

    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        project.service<KritProjectService>().applyFix(KritFixLabels.normalizeFixLevel(fixLevel), findingId)
    }

    override fun startInWriteAction(): Boolean = false
}

class KritApplySuggestionIntention(
    private val findingId: String,
    private val suggestion: KritSuggestedFix,
) : IntentionAction {
    override fun getText(): String = KritFixLabels.suggestionTitle(suggestion)

    override fun getFamilyName(): String = KritFixLabels.SUGGESTION_FAMILY_NAME

    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean = true

    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        project.service<KritProjectService>().applySuggestion(findingId, suggestion.id)
    }

    override fun startInWriteAction(): Boolean = false
}
