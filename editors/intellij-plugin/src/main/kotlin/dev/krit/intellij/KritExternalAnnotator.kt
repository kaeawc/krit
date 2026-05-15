package dev.krit.intellij

import com.intellij.lang.annotation.AnnotationHolder
import com.intellij.lang.annotation.ExternalAnnotator
import com.intellij.lang.annotation.HighlightSeverity
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
        return collectedInfo?.let(KritRunner::analyze).orEmpty()
    }

    override fun apply(file: PsiFile, annotationResult: List<KritFinding>?, holder: AnnotationHolder) {
        for (finding in annotationResult.orEmpty()) {
            holder.newAnnotation(highlightSeverity(finding), finding.message)
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

