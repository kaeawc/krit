package dev.krit.intellij

import com.intellij.codeInspection.LocalInspectionTool
import com.intellij.codeInspection.ProblemHighlightType
import com.intellij.codeInspection.ProblemsHolder
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
                for (finding in KritRunner.analyze(file)) {
                    holder.registerProblem(
                        file,
                        finding.message,
                        problemHighlightType(finding),
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
}

