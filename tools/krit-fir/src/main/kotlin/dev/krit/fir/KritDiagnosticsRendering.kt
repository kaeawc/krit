package dev.krit.fir

import org.jetbrains.kotlin.diagnostics.KtDiagnosticFactoryToRendererMap
import org.jetbrains.kotlin.diagnostics.KtDiagnosticRenderers
import org.jetbrains.kotlin.diagnostics.rendering.BaseDiagnosticRendererFactory

object KritDiagnosticsRendering : BaseDiagnosticRendererFactory() {
    override val MAP by KtDiagnosticFactoryToRendererMap("Krit") { map ->
        // Prefix each message with [DIAGNOSTIC_NAME] so the test harness can distinguish
        // plugin diagnostics from standard Kotlin compiler warnings.
        map.put(
            KritDiagnostics.FLOW_COLLECT_IN_ON_CREATE,
            "[FLOW_COLLECT_IN_ON_CREATE] Flow.collect() called directly in onCreate(). Use lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { collect() } }.",
        )
        map.put(
            KritDiagnostics.COMPOSE_REMEMBER_WITHOUT_KEY,
            "[COMPOSE_REMEMBER_WITHOUT_KEY] remember '{' {0} '}' is missing an explicit key argument.",
            KtDiagnosticRenderers.TO_STRING,
        )
        map.put(
            KritDiagnostics.INJECT_DISPATCHER,
            "[INJECT_DISPATCHER] Hardcoded Dispatchers.{0}. Inject dispatchers for better testability.",
            KtDiagnosticRenderers.TO_STRING,
        )
        map.put(
            KritDiagnostics.UNSAFE_CAST_WHEN_NULLABLE,
            "[UNSAFE_CAST_WHEN_NULLABLE] Unsafe cast to nullable type; prefer 'as?' to avoid ClassCastException at runtime.",
        )
        map.put(
            KritDiagnostics.SMOKE_CLASS,
            "[SMOKE_CLASS] Class named 'Smoke' detected by smoke-test checker.",
        )
    }
}
