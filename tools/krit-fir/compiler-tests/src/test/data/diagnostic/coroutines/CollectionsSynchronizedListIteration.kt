// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15, 19, 25, 28, 36, 37, 38, 39, 40, 41, 42, 43, 45, 46, 47, 48, 54, 55, 56, 57, 58, 59, 61, 67, 70, 74, 77, 87, 91, 96, 102, 108, 112, 123
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

fun chains(list: MutableList<Int>, map: MutableMap<String, Int>, set: MutableSet<Int>) {
    // The wrapper anywhere in the loop header, as Go's text match finds it:
    // a view, an adapter, or a copy that walks the wrapper's iterator.
    <!CollectionsSynchronizedListIteration!>for<!> (key in Collections.synchronizedMap(map).keys) consume(key)
    <!CollectionsSynchronizedListIteration!>for<!> ((index, item) in Collections.synchronizedList(list).withIndex()) consume(index + item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list).filter { it > 0 }) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in run { Collections.synchronizedList(list) }) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in (Collections.synchronizedList(list) as List<Int>)) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list) ?: list) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list).subList(0, 1)) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedSet(set).toSet()) consume(item)
    // A one-element Set is copied through its iterator.
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedSet(set).toList()) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedSet(set).sorted()) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list).stream().iterator()) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list).also { consume(it) }) consume(item)
}

fun iteratingScalars(list: MutableList<Int>, set: MutableSet<Int>) {
    // Calls that yield a String, a number, a Boolean, or Unit, but walk the
    // wrapper's unsynchronized iterator to compute it, as Go reports them.
    <!CollectionsSynchronizedListIteration!>for<!> (char in Collections.synchronizedList(list).joinToString(",")) consume(char)
    <!CollectionsSynchronizedListIteration!>for<!> (index in 0 until Collections.synchronizedList(list).count { it > 0 }) consume(index)
    <!CollectionsSynchronizedListIteration!>for<!> (index in 0 until Collections.synchronizedList(list).sum()) consume(index)
    <!CollectionsSynchronizedListIteration!>for<!> (index in 0 until Collections.synchronizedList(list).maxOf { it }) consume(index)
    <!CollectionsSynchronizedListIteration!>for<!> (item in list.filter { Collections.synchronizedList(list).any { other -> other > it } }) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in run { Collections.synchronizedList(list).forEach { consume(it) }; list }) consume(item)
    // Iterable.first() on a Set takes the wrapper's iterator.
    <!CollectionsSynchronizedListIteration!>for<!> (char in Collections.synchronizedSet(set).first().toString()) consume(char)
}

fun bodyIteration(names: List<String>, nums: MutableList<Int>) {
    // The loop's code iterates an inline wrapper in its body without a lock,
    // as Go's text match of the whole for statement reports.
    <!CollectionsSynchronizedListIteration!>for<!> (id in names) {
        Collections.synchronizedList(nums).forEach { consume(it + id.length) }
    }
    <!CollectionsSynchronizedListIteration!>for<!> (id in names) {
        val wrapped = Collections.synchronizedList(nums)
        consume(wrapped.joinToString(id))
    }
    <!CollectionsSynchronizedListIteration!>for<!> (id in names) consume(Collections.synchronizedMap(mutableMapOf(id to 1)).keys.first())
    // The inner loop iterates a var, which FIR does not treat as holding the
    // wrapper, so the outer loop's finding stands, as in Go.
    <!CollectionsSynchronizedListIteration!>for<!> (id in names) {
        var wrapped = Collections.synchronizedList(nums)
        for (num in wrapped) consume(id + num)
        wrapped = nums
        consume(wrapped)
    }
}

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
