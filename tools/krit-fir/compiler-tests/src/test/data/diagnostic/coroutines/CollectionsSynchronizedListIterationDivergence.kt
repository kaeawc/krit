// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 19, 29, 30, 84, 85, 86
// Where the checker and the Go rule's text match of the whole `for`
// statement disagree. Go reports a loop whose text (header, body, comments,
// and strings) contains `Collections.synchronizedList`, `...Set`, or
// `...Map`; FIR reports a loop whose iterable holds a java.util.Collections
// synchronized wrapper.
package test

import java.util.Collections

// Go reports these because the loop body mentions the wrapper; FIR is
// correct to drop them because the loop iterates `items`, not a wrapper.
fun bodyOnly(items: List<Int>) {
    for (item in items) {
        val wrapped = Collections.synchronizedList(mutableListOf(item))
        consume(wrapped)
    }
    for (item in items) {
        // Collections.synchronizedList is not used here.
        consume("Collections.synchronizedMap $item")
    }
}

// Go reports the outer loop too because its body contains the inner loop's
// header; FIR is correct to report only the inner loop, which iterates the
// wrapper.
fun nestedOuter(lists: List<MutableList<Int>>) {
    for (list in lists) {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list)) consume(item)
    }
}

// Go misses these because the header does not spell the wrapper call; FIR
// reports them because the loop iterates a `val` that holds the wrapper
// (directly or through a lazy view or adapter).
class Registry {
    private val listeners = Collections.synchronizedList(mutableListOf<Runnable>())
    private val byName = Collections.synchronizedMap(mutableMapOf<String, Int>())

    fun dispatch() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
        <!CollectionsSynchronizedListIteration!>for<!> (name in byName.keys) consume(name)
        <!CollectionsSynchronizedListIteration!>for<!> (count in byName.values) consume(count)
        <!CollectionsSynchronizedListIteration!>for<!> ((name, count) in byName.entries) consume(name + count)
        <!CollectionsSynchronizedListIteration!>for<!> ((name, count) in byName) consume(name + count)
        <!CollectionsSynchronizedListIteration!>for<!> ((index, listener) in listeners.withIndex()) consume(index to listener)
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners.asSequence()) consume(listener)
        <!CollectionsSynchronizedListIteration!>for<!> (listener in this.listeners.iterator()) consume(listener)
    }

    fun guarded() {
        synchronized(listeners) {
            for (listener in listeners) listener.run()
        }
    }
}

val topLevelWrapper = Collections.synchronizedSet(mutableSetOf(1))

fun heldValues(holder: Registry) {
    val local = Collections.synchronizedList(mutableListOf(1))
    <!CollectionsSynchronizedListIteration!>for<!> (item in local) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in topLevelWrapper) consume(item)
    consume(holder)
}

// Go misses these because its text match knows only synchronizedList,
// synchronizedSet, and synchronizedMap; FIR reports the other
// Collections.synchronized* wrappers the message names.
fun otherWrappers(values: MutableCollection<Int>) {
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedCollection(values)) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedSortedSet(sortedSetOf(1))) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedNavigableSet(sortedSetOf(1))) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> ((key, value) in Collections.synchronizedSortedMap(sortedMapOf(1 to 2))) consume(key + value)
    <!CollectionsSynchronizedListIteration!>for<!> ((key, value) in Collections.synchronizedNavigableMap(java.util.TreeMap(mapOf(1 to 2)))) consume(key + value)
}

// Go reports these because the header spells the wrapper call; FIR is
// correct to drop them because the loop iterates indices, the filtered
// elements, or a string's characters, and the wrapper only yields a scalar
// (its size, a contains() result, an element's text) read under its own lock.
fun scalarUses(list: MutableList<Int>) {
    for (index in 0 until Collections.synchronizedList(list).size) consume(index)
    for (item in list.filter { Collections.synchronizedList(list).contains(it) }) consume(item)
    for (char in Collections.synchronizedList(list).first().toString()) consume(char)
}

// A call named synchronized after the loop's expression does not guard it:
// the receiver is evaluated before the call runs. Go's tree-sitter parse
// nests a receiver inside the call it qualifies, so Go does not report this;
// FIR does.
fun <T> Any.synchronized(block: () -> T): T = kotlin.synchronized(this, block)

fun receiverOfSynchronized() {
    run {
        <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
        Any()
    }.synchronized { consume(1) }
}

private fun consume(value: Any?) {
    println(value)
}
