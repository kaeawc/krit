// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: loops Go leaves alone, and loops under an enclosing call written
// `synchronized`, whatever the lock, up to the nearest named function.
package test

import java.util.Collections
import java.util.concurrent.CopyOnWriteArrayList

fun externallySynchronized() {
    val list = Collections.synchronizedList(mutableListOf(1, 2, 3))
    synchronized(list) {
        for (item in list) consume(item)
        for (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
    }
}

fun otherLock(lock: Any) {
    // Go accepts any lock, so this does too.
    synchronized(lock) {
        for (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
    }
}

fun qualifiedSynchronized(lock: Any) {
    kotlin.synchronized(lock) {
        for (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
    }
}

fun lambdaInsideSynchronized(lock: Any) {
    // Go does not stop at lambdas, anonymous functions, or object
    // expressions: the enclosing synchronized call counts.
    synchronized(lock) {
        listOf(1).forEach {
            for (item in Collections.synchronizedList(mutableListOf(it))) consume(item)
        }
        val block = fun() {
            for (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
        }
        block()
        object {
            init {
                for (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
            }
        }
    }
}

fun plainCollections(list: List<Int>) {
    val copyOnWrite = CopyOnWriteArrayList(listOf(1, 2, 3))
    for (item in copyOnWrite) consume(item)
    for (item in list) consume(item)
    for (item in Collections.unmodifiableList(list)) consume(item)
}

// A var may no longer hold the wrapper; a getter or delegate computes the
// value elsewhere.
var reassignable: MutableList<Int> = Collections.synchronizedList(mutableListOf(1))
val computed: MutableList<Int> get() = Collections.synchronizedList(mutableListOf(1))
val lazyHeld: MutableList<Int> by lazy { Collections.synchronizedList(mutableListOf(1)) }

fun notHeld() {
    for (item in reassignable) consume(item)
    for (item in computed) consume(item)
    for (item in lazyHeld) consume(item)
    // A snapshot copy iterates a new list, not the wrapper, and indices do
    // not iterate it at all.
    val wrapped = Collections.synchronizedList(mutableListOf(1))
    for (item in wrapped.toList()) consume(item)
    for (index in wrapped.indices) consume(index)
}

private fun consume(value: Any?) {
    println(value)
}
