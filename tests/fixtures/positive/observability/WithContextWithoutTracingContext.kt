package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
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

fun handle(tracer: Tracer) {
    tracer.spanBuilder("handle").startSpan().use {
        runBlocking {
            withContext(Dispatchers.IO) { fetch() }
        }
    }
}

fun fetch() {}
