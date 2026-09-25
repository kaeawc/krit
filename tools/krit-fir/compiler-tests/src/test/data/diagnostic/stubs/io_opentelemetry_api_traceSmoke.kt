// Smoke: start a span, make it current with use {}, and end it in finally.
package stubs

import io.opentelemetry.api.trace.Span
import io.opentelemetry.api.trace.SpanBuilder
import io.opentelemetry.api.trace.SpanKind
import io.opentelemetry.api.trace.StatusCode
import io.opentelemetry.api.trace.Tracer

fun traced(tracer: Tracer) {
    val builder: SpanBuilder = tracer.spanBuilder("work").setSpanKind(SpanKind.INTERNAL).setAttribute("k", "v")
    val span: Span = builder.startSpan()
    try {
        span.makeCurrent().use {
            span.setAttribute("count", 1L)
            span.addEvent("started")
        }
    } catch (e: Exception) {
        span.recordException(e)
        span.setStatus(StatusCode.ERROR)
    } finally {
        span.end()
    }
    <!PrintlnInProduction!>println<!>(Span.current())
}
