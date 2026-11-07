package rules_test

import "testing"

func TestAuditObservabilityLoggers(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"slf4j interpolation", "LoggerInterpolatedMessage", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun f(id: Int) { logger.info("user $id") }
`, 1},
		{"jul interpolation", "LoggerInterpolatedMessage", `
package test
import java.util.logging.Logger
private val logger = Logger.getLogger("x")
fun f(id: Int) { logger.info("user $id") }
`, 0},
		{"kotlin logging lambda", "LoggerInterpolatedMessage", `
package test
import mu.KotlinLogging
private val logger = KotlinLogging.logger {}
fun f(id: Int) { logger.info { "user $id" } }
`, 0},
		{"unrelated receiver in slf4j file concat", "LoggerStringConcat", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
class Banner { fun info(s: String) {} }
fun f(banner: Banner, id: Int) { banner.info("user " + id) }
`, 0},
		{"slf4j concat", "LoggerStringConcat", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun f(id: Int) { logger.info("user " + id) }
`, 1},
		{"concat of two literals", "LoggerStringConcat", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun f() { logger.info("user " + "id") }
`, 0},
		{"throwable word boundary", "UnstructuredErrorLog", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun f(total: Int) { logger.error("can't load $total") }
`, 0},
		{"throwable interpolation", "UnstructuredErrorLog", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun f(e: Exception) { logger.error("failed: $e") }
`, 1},
		{"trace id interpolation", "TraceIdLoggedAsPlainMessage", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun f(traceId: String) { logger.info("trace=$traceId") }
`, 1},
		{"get logger in function", "LoggerWithoutLoggerField", `
package test
import org.slf4j.LoggerFactory
fun f() { val l = LoggerFactory.getLogger("x"); l.info("a") }
`, 1},
		{"get logger in companion", "LoggerWithoutLoggerField", `
package test
import org.slf4j.LoggerFactory
class A { companion object { private val log = LoggerFactory.getLogger(A::class.java) } }
`, 0},
		{"get logger lazy in function", "LoggerWithoutLoggerField", `
package test
import org.slf4j.LoggerFactory
class A { val log by lazy { LoggerFactory.getLogger(A::class.java) } }
`, 0},
		{"mdc put with remove in finally", "MdcPutNoRemove", `
package test
import org.slf4j.MDC
fun f() { MDC.put("k", "v"); try { println() } finally { MDC.remove("k") } }
`, 0},
		{"mdc put leaks", "MdcPutNoRemove", `
package test
import org.slf4j.MDC
fun f() { MDC.put("k", "v"); println() }
`, 1},
		{"mdc put with clear", "MdcPutNoRemove", `
package test
import org.slf4j.MDC
fun f() { MDC.put("k", "v"); println(); MDC.clear() }
`, 0},
		{"mdc put remove other key", "MdcPutNoRemove", `
package test
import org.slf4j.MDC
fun f() { MDC.put("k", "v"); println(); MDC.remove("other") }
`, 1},
		{"mdc across coroutine", "MdcAcrossCoroutineBoundary", `
package test
import org.slf4j.MDC
import kotlinx.coroutines.*
fun f(scope: CoroutineScope) { MDC.put("k", "v"); scope.launch { println() } }
`, 1},
		{"mdc across coroutine with context", "MdcAcrossCoroutineBoundary", `
package test
import org.slf4j.MDC
import kotlinx.coroutines.*
import kotlinx.coroutines.slf4j.MDCContext
fun f(scope: CoroutineScope) { MDC.put("k", "v"); scope.launch(MDCContext()) { println() } }
`, 0},
		{"mdc put then builder in other function", "MdcAcrossCoroutineBoundary", `
package test
import org.slf4j.MDC
import kotlinx.coroutines.*
fun f() { MDC.put("k", "v") }
fun g(scope: CoroutineScope) { scope.launch { println() } }
`, 0},
		{"log without correlation", "LogWithoutCorrelationId", `
package test
import org.slf4j.LoggerFactory
import kotlinx.coroutines.*
private val logger = LoggerFactory.getLogger("x")
fun f(scope: CoroutineScope) { scope.launch { logger.info("a") } }
`, 1},
	})
}

func TestAuditObservabilityMetricsAndTracing(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"kotlin concurrent timer", "MetricNameMissingUnit", `
package test
import kotlin.concurrent.timer
fun f() { timer("poll", period = 1000L) { println() } }
`, 0},
		{"micrometer counter missing unit", "MetricNameMissingUnit", `
package test
import io.micrometer.core.instrument.MeterRegistry
fun f(registry: MeterRegistry) { registry.counter("orders_created") }
`, 1},
		{"micrometer dotted counter", "MetricNameMissingUnit", `
package test
import io.micrometer.core.instrument.MeterRegistry
fun f(registry: MeterRegistry) { registry.counter("orders.created") }
`, 0},
		{"stepper negative increment", "MetricCounterNotMonotonic", `
package test
class Stepper { fun increment(by: Int) {} }
fun f(stepper: Stepper) { stepper.increment(-1) }
`, 0},
		{"counter negative increment", "MetricCounterNotMonotonic", `
package test
class Counter { fun increment(by: Long) {} }
fun f(requestCounter: Counter) { requestCounter.increment(-1L) }
`, 1},
		{"span ended in callback", "SpanStartWithoutFinish", `
package test
import io.opentelemetry.api.trace.Tracer
fun f(tracer: Tracer, run: (() -> Unit) -> Unit) {
    val span = tracer.spanBuilder("x").startSpan()
    run { span.end() }
}
`, 0},
		{"span returned", "SpanStartWithoutFinish", `
package test
import io.opentelemetry.api.trace.Span
import io.opentelemetry.api.trace.Tracer
fun f(tracer: Tracer): Span {
    val span = tracer.spanBuilder("x").startSpan()
    return span
}
`, 0},
		{"span leaked", "SpanStartWithoutFinish", `
package test
import io.opentelemetry.api.trace.Tracer
fun f(tracer: Tracer) {
    val span = tracer.spanBuilder("x").startSpan()
    println(span)
}
`, 0},
		{"span never ended", "SpanStartWithoutFinish", `
package test
import io.opentelemetry.api.trace.Tracer
fun f(tracer: Tracer) {
    val span = tracer.spanBuilder("x").startSpan()
    span.setAttribute("a", "b")
}
`, 1},
		{"span attribute on dom", "SpanAttributeWithHighCardinality", `
package test
class El { fun setAttribute(k: String, v: String) {} }
fun f(el: El) { el.setAttribute("user_id", "x") }
`, 0},
		{"span attribute high card", "SpanAttributeWithHighCardinality", `
package test
import io.opentelemetry.api.trace.Span
fun f(span: Span) { span.setAttribute("user_id", "x") }
`, 1},
	})
}

func TestAuditObservabilityNestedMdcContext(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"nested withContext inherits mdc", "LogWithoutCorrelationId", `
package test
import org.slf4j.LoggerFactory
import kotlinx.coroutines.*
import kotlinx.coroutines.slf4j.MDCContext
private val logger = LoggerFactory.getLogger("x")
fun f(scope: CoroutineScope) {
    scope.launch(MDCContext()) {
        withContext(Dispatchers.IO) { logger.info("a") }
    }
}
`, 0},
	})
}
