package test

interface Tracer {
    fun spanBuilder(name: String): SpanBuilder
}

interface SpanBuilder {
    fun startSpan(): Span
}

interface Span {
    fun end()
}

fun handle(tracer: Tracer) {
    val span = tracer.spanBuilder("handle").startSpan()
    doWork()
}

fun doWork() {}
