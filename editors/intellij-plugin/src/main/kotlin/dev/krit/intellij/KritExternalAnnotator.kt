package dev.krit.intellij

import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.components.service
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
            holder.newAnnotation(highlightSeverity(finding), finding.displayMessage)
                .range(KritRanges.rangeFor(file, finding))
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
