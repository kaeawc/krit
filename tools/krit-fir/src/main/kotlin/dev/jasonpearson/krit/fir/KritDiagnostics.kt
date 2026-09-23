package dev.jasonpearson.krit.fir

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.diagnostics.KtDiagnosticFactory0
import org.jetbrains.kotlin.diagnostics.KtDiagnosticFactory1
import org.jetbrains.kotlin.diagnostics.KtDiagnosticsContainer
import org.jetbrains.kotlin.diagnostics.rendering.BaseDiagnosticRendererFactory
import org.jetbrains.kotlin.diagnostics.warning0
import org.jetbrains.kotlin.diagnostics.warning1

object KritDiagnostics : KtDiagnosticsContainer() {
    val FLOW_COLLECT_IN_ON_CREATE: KtDiagnosticFactory0 by warning0<PsiElement>()
    val COMPOSE_REMEMBER_WITHOUT_KEY: KtDiagnosticFactory1<String> by warning1<PsiElement, String>()
    val INJECT_DISPATCHER: KtDiagnosticFactory1<String> by warning1<PsiElement, String>()
    // warning, not error: the checker flags a code smell (prefer `as?`), not a
    // guaranteed failure, matching the other krit checkers and the
    // potentialbugs ruleset. Emitting compiler-error severity would mislabel
    // legitimate-but-risky casts.
    val UNSAFE_CAST_WHEN_NULLABLE: KtDiagnosticFactory0 by warning0<PsiElement>()
    val SMOKE_CLASS: KtDiagnosticFactory0 by warning0<PsiElement>()

    override fun getRendererFactory(): BaseDiagnosticRendererFactory = KritDiagnosticsRendering
}
