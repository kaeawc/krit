package test

import io.opentelemetry.context.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

interface Tracer {
    fun spanBuilder(name: String): SpanBuilder
}

interface SpanBuilder {
    fun startSpan(): Span
}

interface Span {
    fun close()
}

fun <T : Span, R> T.use(block: (T) -> R): R = block(this)
fun Context.asContextElement(): Any = this

fun propagated(tracer: Tracer) {
    tracer.spanBuilder("handle").startSpan().use {
        withContext(Dispatchers.IO + Context.current().asContextElement()) { fetch() }
    }
}

suspend fun noSpan() {
    withContext(Dispatchers.IO) { fetch() }
}

fun fetch() {}
