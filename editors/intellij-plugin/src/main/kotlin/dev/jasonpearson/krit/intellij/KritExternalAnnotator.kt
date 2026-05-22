package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.codeInspection.ProblemHighlightType
import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.lang.annotation.HighlightSeverity
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

    override fun apply(file: PsiFile, annotationResult: List<KritFinding>?, holder: AnnotationHolder) {
        for (finding in annotationResult.orEmpty()) {
            val annotation = holder.newAnnotation(highlightSeverity(finding), finding.displayMessage)
                .range(KritRanges.rangeFor(file, finding))
            for (intention in KritIntentions.forFinding(finding)) {
                annotation.withFix(intention)
            }
            annotation.create()
        }
    }

    private fun highlightSeverity(finding: KritFinding): HighlightSeverity =
        KritSeverity.highlightSeverity(finding)
}

internal object KritSeverity {
    // Confidence in (0, 0.5) is a "soft" finding — render at weak warning so
    // it doesn't compete with high-confidence diagnostics. Confidence == 0
    // means the rule didn't set it; treat as a normal warning.
    private const val WEAK_THRESHOLD = 0.5

    fun highlightSeverity(finding: KritFinding): HighlightSeverity = when {
        finding.severity.equals("error", ignoreCase = true) -> HighlightSeverity.ERROR
        finding.severity.equals("info", ignoreCase = true) -> HighlightSeverity.INFORMATION
        finding.confidence > 0.0 && finding.confidence < WEAK_THRESHOLD -> HighlightSeverity.WEAK_WARNING
        else -> HighlightSeverity.WARNING
    }

    fun problemHighlightType(finding: KritFinding): ProblemHighlightType = when {
        finding.severity.equals("error", ignoreCase = true) ->
            ProblemHighlightType.ERROR
        finding.severity.equals("info", ignoreCase = true) ->
            ProblemHighlightType.INFORMATION
        finding.confidence > 0.0 && finding.confidence < WEAK_THRESHOLD ->
            ProblemHighlightType.WEAK_WARNING
        else -> ProblemHighlightType.WARNING
    }
}

object KritIntentions {
    // Suggestions and the autofix slot are mutually exclusive per finding:
    // when a rule emits suggestions the user picks one explicitly, so the
    // catch-all "apply auto-fixes" entry would conflict. Suppress is
    // orthogonal and always offered.
    fun forFinding(finding: KritFinding): List<IntentionAction> {
        val suppress = KritSuppressIntention(finding.rule)
        val applicable = finding.suggestedFixes.filter { it.edits.isNotEmpty() }
        if (applicable.isNotEmpty()) {
            return applicable.map { KritApplySuggestionIntention(finding.findingId, it) } + suppress
        }
        if (finding.fixable) {
            return listOf(KritApplyFixesIntention(finding.fixLevel), suppress)
        }
        return listOf(suppress)
    }
}

class KritApplyFixesIntention(private val fixLevel: String?) : IntentionAction {
    override fun getText(): String = KritFixLabels.applyFixesTitle(fixLevel)

    override fun getFamilyName(): String = text

    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean = true

    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        project.service<KritProjectService>().applyFixes(KritFixLabels.normalizeFixLevel(fixLevel))
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
