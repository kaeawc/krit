// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: the result is used, the type is not must-use, or the call is exempt.
package test

import com.google.errorprone.annotations.CanIgnoreReturnValue
import com.google.errorprone.annotations.CheckReturnValue
import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.processors.PublishProcessor
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

class Counter {
    @CheckReturnValue
    @CanIgnoreReturnValue
    fun increment(): Int = 1

    @CheckReturnValue
    fun peek(): Int = 2
}

fun sequenceOfSquares(items: Sequence<Int>): Sequence<Int> = items.map { it * it }

fun assigned(items: Sequence<Int>) {
    val mapped = items.map { it + 1 }
    println(mapped.count())
}

fun returned(items: Sequence<Int>): Sequence<Int> {
    return items.map { it + 1 }
}

fun passed(items: Sequence<Int>) {
    println(items.map { it + 1 }.toList())
}

fun lastStatementOfLambda(items: Sequence<Int>): Sequence<Int> = run {
    println("start")
    items.map { it + 1 }
}

// The last statement of a lambda is its value, even when the lambda is coerced to Unit.
fun coercedLastStatement(outer: List<Int>, items: Sequence<Int>) {
    outer.forEach { items.map { x -> x + it } }
}

fun coercedTails(outer: List<Int>, items: Sequence<Int>, flag: Boolean) {
    outer.forEach { if (flag) items.map { x -> x + it } else items.filter { x -> x > it } }
    outer.forEach {
        try {
            items.map { x -> x + it }
        } catch (e: Exception) {
            items.filter { x -> x > it }
        }
    }
    outer.forEach {
        when (flag) {
            true -> items.map { x -> x + it }
            false -> items.filter { x -> x > it }
        }
    }
}

fun usedByElvis(items: Sequence<Int>?, fallback: Sequence<Int>) {
    val chosen = items?.map { it + 1 } ?: fallback
    println(chosen.count())
}

fun usedAsBranchValue(flag: Boolean, items: Sequence<Int>) {
    val chosen = if (flag) items.map { it + 1 } else items.filter { it > 1 }
    println(chosen.count())
}

fun usedByPropertyRead(items: Sequence<Int>) {
    items.map { it + 1 }.iterator()
}

// A List result is not a lazy builder, and neither is a terminal operation.
fun eagerResults(items: List<Int>, sequence: Sequence<Int>) {
    items.map { it + 1 }
    sequence.toList()
    sequence.count()
}

// The annotation is overridden by @CanIgnoreReturnValue.
fun ignorable(counter: Counter) {
    counter.increment()
}

// An unannotated callee returning a project type.
class Box {
    fun items(): Sequence<Int> = sequenceOf(1)
}

fun projectCallee(box: Box) {
    box.items()
}

// Calls named like a stdlib builder on a project type that returns Unit.
class Sink {
    fun map(block: (Int) -> Unit): Unit = block(1)
    fun filter(block: (Int) -> Boolean): Unit { block(1) }
}

fun unitLookalike(sink: Sink) {
    sink.map { println(it) }
    sink.filter { it > 0 }
}

// A project function that returns a Flow is not a stdlib builder.
fun projectFlow(source: Flow<Int>): Flow<Int> = source.map { it + 1 }

fun discardedProjectFlow(source: Flow<Int>) {
    projectFlow(source)
}

// A function value is invoked, not built.
fun invoke(callback: (Int) -> Unit, supplier: () -> Sequence<Int>) {
    callback(1)
    supplier()
}

// In-place mutators and collection mutators keep their result for chaining only.
fun mutators(items: MutableList<Int>, map: MutableMap<String, Int>) {
    items.sort()
    items.shuffle()
    items.reverse()
    items.fill(0)
    items.clear()
    items.add(1)
    items.remove(1)
    map.put("a", 1)
}

// Assertion helpers return their argument for chaining.
@CheckReturnValue
fun assertPositive(value: Int): Int = value

fun assertions() {
    assertPositive(1)
}

// `subscribe` with handlers and `offer` in a verification chain are fire and forget.
fun rxHandlers(source: Observable<String>) {
    source.subscribe { value -> println(value) }
}

fun verified(processor: PublishProcessor<String>) {
    verify(processor).offer("a")
}

fun <T> verify(mock: T): T = mock

// A scope function yields its lambda value; the lambda decides.
fun scopeFunctions(items: Sequence<Int>) {
    items.let { println(it.count()) }
    items.also { println(it.count()) }
}

// A lambda that returns from the enclosing function may never complete the call.
@CheckReturnValue
inline fun <T> checked(block: () -> T): T = block()

fun nonLocalReturn(flag: Boolean): Int {
    checked { if (flag) return 1 else 2 }
    return 0
}

// A local return only leaves the lambda, so the call still completes and is reported elsewhere.
fun usedValue(items: Sequence<Int>): Int = items.map { it + 1 }.count()
