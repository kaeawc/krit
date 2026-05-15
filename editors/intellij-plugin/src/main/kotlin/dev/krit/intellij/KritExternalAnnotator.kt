package dev.krit.intellij

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
            if (finding.fixable) {
                annotation.withFix(KritApplyFixesIntention(finding.fixLevel))
            }
            annotation
                .create()
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

class KritApplyFixesIntention(private val fixLevel: String?) : IntentionAction {
    override fun getText(): String = "Apply Krit ${normalizedFixLevel()} auto-fixes"

    override fun getFamilyName(): String = text

    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean = true

    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        project.service<KritProjectService>().applyFixes(normalizedFixLevel())
    }

    override fun startInWriteAction(): Boolean = false

    private fun normalizedFixLevel(): String {
        return fixLevel.orEmpty().ifBlank { "idiomatic" }
    }
}
