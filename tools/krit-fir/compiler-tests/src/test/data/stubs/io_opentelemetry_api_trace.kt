// Compiler-test source stubs; never packaged in the production artifact.
package io.opentelemetry.api.trace

import io.opentelemetry.context.Context
import io.opentelemetry.context.ImplicitContextKeyed

interface Tracer {
    fun spanBuilder(spanName: String): SpanBuilder
}

interface SpanBuilder {
    fun setParent(context: Context): SpanBuilder

    fun setNoParent(): SpanBuilder

    fun setAttribute(key: String, value: String): SpanBuilder

    fun setAttribute(key: String, value: Long): SpanBuilder

    fun setAttribute(key: String, value: Boolean): SpanBuilder

    fun setSpanKind(spanKind: SpanKind): SpanBuilder

    fun startSpan(): Span
}

interface Span : ImplicitContextKeyed {
    fun setAttribute(key: String, value: String?): Span

    fun setAttribute(key: String, value: Long): Span

    fun setAttribute(key: String, value: Boolean): Span

    fun addEvent(name: String): Span

    fun setStatus(statusCode: StatusCode): Span

    fun setStatus(statusCode: StatusCode, description: String): Span

    fun recordException(exception: Throwable): Span

    fun updateName(name: String): Span

    fun end()

    val isRecording: Boolean

    // Java interface statics; callable ids gain `.Companion` (see README).
    companion object {
        fun current(): Span = TODO()

        fun fromContext(context: Context): Span = TODO()

        fun getInvalid(): Span = TODO()
    }
}

enum class StatusCode {
    UNSET,
    OK,
    ERROR,
}

enum class SpanKind {
    INTERNAL,
    SERVER,
    CLIENT,
    PRODUCER,
    CONSUMER,
}
