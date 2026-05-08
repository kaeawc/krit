package test

interface Tracer {
    fun spanBuilder(name: String): SpanBuilder
}

interface SpanBuilder {
    fun startSpan(): Span
}

interface Span {
    fun end()
    fun makeCurrent(): Scope
}

interface Scope

fun <T : Span, R> T.use(block: (T) -> R): R = block(this)
fun <T : Scope, R> T.use(block: (T) -> R): R = block(this)

fun handle(tracer: Tracer) {
    val span = tracer.spanBuilder("handle").startSpan()
    try {
        doWork()
    } finally {
        span.end()
    }
}

fun scoped(tracer: Tracer) {
    val span = tracer.spanBuilder("scoped").startSpan()
    span.makeCurrent().use {
        doWork()
    }
}

fun assignedUse(tracer: Tracer) {
    val span = tracer.spanBuilder("assigned").startSpan()
    span.use {
        doWork()
    }
}

fun doWork() {}
