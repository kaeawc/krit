// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Recall: values detekt reads as discarded that Go reads as used. Go keeps the last statement of every
// lambda as the lambda's value, but a lambda coerced to Unit drops it, and so does a `let`, `run` or `with`
// whose own result is discarded.
package test

import com.google.errorprone.annotations.CanIgnoreReturnValue
import com.google.errorprone.annotations.CheckReturnValue
import io.reactivex.rxjava3.processors.PublishProcessor

fun coercedLastStatement(outer: List<Int>, items: Sequence<Int>) {
    outer.forEach { items.<!IgnoredReturnValue!>map<!> { x -> x + it } }
}

fun coercedTails(outer: List<Int>, items: Sequence<Int>, flag: Boolean) {
    outer.forEach { if (flag) items.<!IgnoredReturnValue!>map<!> { x -> x + it } else items.<!IgnoredReturnValue!>filter<!> { x -> x > it } }
    outer.forEach {
        try {
            items.<!IgnoredReturnValue!>map<!> { x -> x + it }
        } catch (e: Exception) {
            items.<!IgnoredReturnValue!>filter<!> { x -> x > it }
        }
    }
    outer.forEach {
        when (flag) {
            true -> items.<!IgnoredReturnValue!>map<!> { x -> x + it }
            false -> items.<!IgnoredReturnValue!>filter<!> { x -> x > it }
        }
    }
}

// A callback typed `() -> Unit` drops the processor's delivery result.
fun callback(processor: PublishProcessor<Int>): () -> Unit = { processor.<!IgnoredReturnValue!>offer<!>(1) }

fun alsoTail(items: Sequence<Int>) {
    items.also { it.<!IgnoredReturnValue!>map<!> { x -> x + 1 } }
}

// The lambda value of a discarded `let`, `run` or `with` is discarded with it.
fun scopeTails(items: Sequence<Int>, maybe: Sequence<Int>?, processor: PublishProcessor<Int>) {
    items.let { it.<!IgnoredReturnValue!>map<!> { x -> x + 1 } }
    maybe?.let { processor.<!IgnoredReturnValue!>offer<!>(1) }
    run { items.<!IgnoredReturnValue!>filter<!> { it > 0 } }
    with(items) { <!IgnoredReturnValue!>map<!> { it + 1 } }
    items.run { let { it.<!IgnoredReturnValue!>map<!> { x -> x } } }
    items.let { return@let it.<!IgnoredReturnValue!>map<!> { x -> x } }
}

// The value is used: the scope call's result is read, or the lambda ends with something else.
fun scopeTailsUsed(items: Sequence<Int>): Int {
    val mapped = items.let { it.map { x -> x + 1 } }
    items.let { println(it.count()) }
    items.let { it }
    return mapped.count() + run { items.map { it + 1 } }.count()
}

// errorprone's class-level @CheckReturnValue covers the class's methods and constructors unless one
// opts out, as detekt reads it from the callee's containing declaration.
@CheckReturnValue
class Builder {
    fun size(): Int = 1

    @CanIgnoreReturnValue
    fun add(value: Int): Builder = this
}

@CheckReturnValue
@CanIgnoreReturnValue
class Lenient {
    fun size(): Int = 1
}

fun classAnnotated(builder: Builder, lenient: Lenient) {
    builder.<!IgnoredReturnValue!>size<!>()
    <!IgnoredReturnValue!>Builder<!>()
    builder.add(1)
    lenient.size()
    val used = builder.size()
    println(used)
}
