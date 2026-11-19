// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: Boolean properties, names without the `is` + uppercase prefix,
// types detekt treats as Boolean or as unknown, and variables that are not
// properties. Go reports none of these either.
package test

import java.util.concurrent.atomic.AtomicBoolean
import kotlin.reflect.KFunction0

typealias Predicate = () -> Boolean

typealias Matcher<A> = (A) -> Boolean

fun ready(): Boolean = true

class Flags(val enabled: Boolean) {
    val isEnabled: Boolean = true

    var isVisible: Boolean? = null

    val isInferredTrue = true

    val isInferredFalse = false

    val isComputed = enabled && isEnabled

    val isGetter: Boolean
        get() = enabled

    val isInferredGetter
        get() = !enabled

    val isLazy by lazy { enabled }

    val isLazyTyped: Boolean by lazy { enabled }

    // detekt's prefix is `is` followed by an uppercase letter.
    val issues: List<String> = emptyList()

    val isolated: Int = 1

    val is_open: String = "o"

    val is2: String = "two"

    val isnt: String = "n"

    val IsCapital: String = "cap"

    val name: String = "name"

    val visible: Int = 0

    // A function type returning Boolean is a predicate, which detekt accepts.
    val isPredicate: () -> Boolean = { true }

    val isNullablePredicate: (() -> Boolean)? = null

    val isNullableResult: () -> Boolean? = { null }

    val isMatcher: (String) -> Boolean = { true }

    val isReceiverPredicate: String.() -> Boolean = { true }

    val isSuspendPredicate: suspend () -> Boolean = { true }

    val isAliasedPredicate: Predicate = { true }

    val isGenericAlias: Matcher<Int> = { true }

    val isFunctionInterface: Function1<Int, Boolean> = { true }

    val isReference: KFunction0<Boolean> = ::ready

    val isInferredReference = ::ready

    val isAnonymousFunction = fun(): Boolean = true

    // detekt treats AtomicBoolean as Boolean.
    val isRunning: AtomicBoolean = AtomicBoolean(false)

    val isStarted = AtomicBoolean(false)

    val isAtomicPredicate: () -> AtomicBoolean = { AtomicBoolean() }

    // A function named like a property is not a property.
    fun isReady(): String = "ready"
}

// A type parameter is not reported: its type is unknown.
class Box<T>(private val value: T) {
    val isValue: T = value

    val isNullableValue: T? = null

    val isDefinitelyNotNull: T & Any
        get() = value!!

    fun <R> local(r: R): R {
        val isLocal: R = r
        return isLocal
    }
}

val <E> List<E>.isFirst: E
    get() = first()

class IsWrapper(val value: String)

fun notProperties(items: List<String>, pairs: List<Pair<String, Int>>, isParam: String): String {
    // Loop variables are not properties...
    for (isItem in items) {
        println(isItem)
    }
    // ...neither are destructuring entries...
    val (isFirst, isSecond) = pairs.first()
    for ((isKey, isValue) in pairs) {
        println(isKey + isValue)
    }
    // ...lambda parameters...
    items.forEach { isElement -> println(isElement) }
    pairs.forEach { (isLeft, isRight) -> println(isLeft + isRight) }
    // ...or catch parameters.
    try {
        println(isParam)
    } catch (isError: IllegalStateException) {
        println(isError)
    }
    return isFirst + isSecond
}
