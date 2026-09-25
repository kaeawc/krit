package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.diagnostics.KtDiagnosticFactoryToRendererMap
import org.jetbrains.kotlin.diagnostics.KtDiagnosticRenderers
import org.jetbrains.kotlin.diagnostics.rendering.BaseDiagnosticRendererFactory

object KritDiagnosticsRendering : BaseDiagnosticRendererFactory() {
    override val MAP by KtDiagnosticFactoryToRendererMap("Krit") { map ->
        map.put(KritDiagnostics.KRIT_RULE, "[{0}] {1}", KtDiagnosticRenderers.TO_STRING, KtDiagnosticRenderers.TO_STRING)
        map.put(
            KritDiagnostics.SMOKE_CLASS,
            "[SMOKE_CLASS] Class named 'Smoke' detected by smoke-test checker.",
        )
    }
}
