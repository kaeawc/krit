// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15, 19, 25, 28, 36, 37, 38, 39, 40, 41, 42, 46, 50, 55, 61, 67, 71, 82
// Positive: a for loop whose iterable holds a java.util.Collections
// synchronized wrapper, outside any synchronized(...) call, should trigger
// CollectionsSynchronizedListIteration, as Go reports it.
package test

import java.util.Collections

fun wrappers() {
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1, 2, 3))) {
        consume(item)
    }

    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedSet(mutableSetOf("a", "b"))) {
        consume(item)
    }

    <!CollectionsSynchronizedListIteration!>for<!> ((key, value) in Collections.synchronizedMap(mutableMapOf("a" to 1))) {
        consume(key)
        consume(value)
    }

    // Fully qualified.
    <!CollectionsSynchronizedListIteration!>for<!> (item in java.util.Collections.synchronizedList(mutableListOf(1))) consume(item)

    // A labelled loop.
    outer@ <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) {
        if (item == 0) break@outer
    }
}

fun chains(list: MutableList<Int>, map: MutableMap<String, Int>) {
    // The wrapper anywhere in the loop header, as Go's text match finds it:
    // a view, an adapter, or a copy made from the wrapper.
    <!CollectionsSynchronizedListIteration!>for<!> (key in Collections.synchronizedMap(map).keys) consume(key)
    <!CollectionsSynchronizedListIteration!>for<!> ((index, item) in Collections.synchronizedList(list).withIndex()) consume(index + item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list).filter { it > 0 }) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in run { Collections.synchronizedList(list) }) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in (Collections.synchronizedList(list) as List<Int>)) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list).toTypedArray()) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list) ?: list) consume(item)}

class Holder {
    fun member() {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
    }

    val initialized = run {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
        1
    }

    init {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
    }
}

object Singleton {
    fun member() {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedSet(mutableSetOf(1))) consume(item)
    }
}

fun lambdas() {
    listOf(1).forEach {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(it))) consume(item)
    }
    val anonymous = object : Runnable {
        override fun run() {
            <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
        }
    }
    anonymous.run()
}

// A named function stops Go's walk for synchronized, so a local function
// declared inside a synchronized block is not covered by it.
fun localFunction(lock: Any) {
    synchronized(lock) {
        fun iterate() {
            <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
        }
        iterate()
    }
}

private fun consume(value: Any?) {
    println(value)
}
