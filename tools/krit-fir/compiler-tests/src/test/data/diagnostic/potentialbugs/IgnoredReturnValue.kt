// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 32, 36, 40, 44, 48, 82x2, 84, 88, 90, 100, 108, 116, 124
// Positives: a discarded call that builds a lazy Sequence, Flow or function value, and a discarded call
// whose callee carries a must-use annotation.
package test

import androidx.annotation.CheckResult
import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.processors.PublishProcessor
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.onEach

class Repository {
    @CheckResult
    fun load(): String = "x"

    @javax.annotation.CheckReturnValue
    fun fetch(): Int = 1

    @com.google.errorprone.annotations.CheckReturnValue
    fun build(): Int = 2
}

fun lazySequence(items: List<Int>) {
    items.asSequence().<!IgnoredReturnValue!>map<!> { it + 1 }
}

fun lazySequenceFilter(items: Sequence<Int>) {
    items.<!IgnoredReturnValue!>filter<!> { it > 1 }
}

fun lazyFlow(source: Flow<Int>) {
    source.<!IgnoredReturnValue!>map<!> { it + 1 }
}

fun lazyFlowOnEach(source: Flow<Int>) {
    source.<!IgnoredReturnValue!>onEach<!> { println(it) }
}

fun flowFactory() {
    <!IgnoredReturnValue!>flowOf<!>(1, 2)
}

fun stateFlowFactory() {
    <!IgnoredReturnValue!>MutableStateFlow<!>(0)
}

fun androidxCheckResult(repository: Repository) {
    repository.<!IgnoredReturnValue!>load<!>()
}

fun jsr305CheckReturnValue(repository: Repository) {
    repository.<!IgnoredReturnValue!>fetch<!>()
}

fun errorProneCheckReturnValue(repository: Repository) {
    repository.<!IgnoredReturnValue!>build<!>()
}

fun rxProcessorOffer(processor: PublishProcessor<String>) {
    processor.<!IgnoredReturnValue!>offer<!>("a")
}

fun rxSubscribeWithoutHandlers(source: Observable<String>) {
    source.<!IgnoredReturnValue!>subscribe<!>()
}

fun rxChainedOperator(source: Observable<String>) {
    source.<!IgnoredReturnValue!>doOnSubscribe<!> { }
}

// A safe call is still a discarded call.
fun safeCall(items: List<Int>?) {
    items?.asSequence()?.<!IgnoredReturnValue!>map<!> { it + 1 }
}

// A branch of a statement-position `if`, `when` and `try`, and a loop body, discard their value.
fun inBranches(flag: Boolean, items: Sequence<Int>) {
    if (flag) items.<!IgnoredReturnValue!>map<!> { it + 1 } else items.<!IgnoredReturnValue!>filter<!> { it > 1 }
    when (flag) {
        true -> items.<!IgnoredReturnValue!>map<!> { it + 2 }
        false -> {}
    }
    try {
        items.<!IgnoredReturnValue!>map<!> { it + 3 }
    } catch (e: Exception) {
        items.<!IgnoredReturnValue!>map<!> { it + 4 }
    }
    for (i in 0..1) {
        items.<!IgnoredReturnValue!>map<!> { it + i }
    }
}

// Not the last statement of the lambda, so its value is dropped.
fun beforeLastStatement(items: Sequence<Int>) {
    run {
        items.<!IgnoredReturnValue!>map<!> { it + 1 }
        println("done")
    }
}

// A callback lambda is coerced to Unit, and every statement before its last one is discarded.
fun callbackStatements(items: Sequence<Int>, outer: List<Int>) {
    outer.forEach {
        items.<!IgnoredReturnValue!>map<!> { x -> x + it }
        println(it)
    }
}

fun insideApply(items: Sequence<Int>, flag: Boolean) {
    StringBuilder().apply {
        if (flag) {
            items.<!IgnoredReturnValue!>filter<!> { it > 1 }
            append("x")
        }
    }
}

// A multi-line chain reports on the line where the chain starts, as Go does.
fun chain(items: Sequence<Int>) {
    <!IgnoredReturnValue!>items<!>
        .filter { it > 0 }
        .map { it + 1 }
}
