package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.components.service
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiFile

class KritExternalAnnotator : ExternalAnnotator<PsiFile, List<KritFinding>>() {
    override fun collectInformation(file: PsiFile): PsiFile? {
        val path = file.virtualFile?.path ?: return null
        if (!path.endsWith(".kt") && !path.endsWith(".kts")) {
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

    private fun highlightSeverity(finding: KritFinding): HighlightSeverity {
        return when (finding.severity.lowercase()) {
            "error" -> HighlightSeverity.ERROR
            "info" -> HighlightSeverity.INFORMATION
            else -> HighlightSeverity.WARNING
        }
    }
}

object KritIntentions {
    // Suggestions and the autofix slot are mutually exclusive per finding:
    // when a rule emits suggestions the user picks one explicitly, so the
    // catch-all "apply auto-fixes" entry would conflict.
    fun forFinding(finding: KritFinding): List<IntentionAction> {
        val applicable = finding.suggestedFixes.filter { it.edits.isNotEmpty() }
        if (applicable.isNotEmpty()) {
            return applicable.map { KritApplySuggestionIntention(finding.findingId, it) }
        }
        if (finding.fixable) {
            return listOf(KritApplyFixesIntention(finding.fixLevel))
        }
        return emptyList()
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
