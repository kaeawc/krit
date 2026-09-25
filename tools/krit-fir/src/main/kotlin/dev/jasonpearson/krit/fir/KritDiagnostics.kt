package dev.jasonpearson.krit.fir

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.diagnostics.KtDiagnosticFactory0
import org.jetbrains.kotlin.diagnostics.KtDiagnosticsContainer
import org.jetbrains.kotlin.diagnostics.rendering.BaseDiagnosticRendererFactory
import org.jetbrains.kotlin.diagnostics.warning0
import org.jetbrains.kotlin.diagnostics.KtDiagnosticFactory2
import org.jetbrains.kotlin.diagnostics.warning2

object KritDiagnostics : KtDiagnosticsContainer() {
    val KRIT_RULE: KtDiagnosticFactory2<String, String> by warning2<PsiElement, String, String>()
    val SMOKE_CLASS: KtDiagnosticFactory0 by warning0<PsiElement>()

    override fun getRendererFactory(): BaseDiagnosticRendererFactory = KritDiagnosticsRendering
}
