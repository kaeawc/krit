// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 26, 34, 41, 46, 57, 58, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128
// Where the checker and the Go rule's text match of the whole `for`
// statement disagree. Go reports a loop whose text (header, body, comments,
// and strings) contains `Collections.synchronizedList`, `...Set`, or
// `...Map`; FIR reports a loop whose iterable holds a java.util.Collections
// synchronized wrapper.
package test

import java.util.Collections

// Go reports these because the loop body mentions the wrapper; FIR is
// correct to drop them because the loop iterates `items`, and its body only
// creates, passes, or reads the wrapper under its own lock, never iterating
// it. (A body that iterates the wrapper is reported: see bodyIteration in
// CollectionsSynchronizedListIteration.kt.)
fun bodyOnly(items: List<Int>) {
    for (item in items) {
        val wrapped = Collections.synchronizedList(mutableListOf(item))
        consume(wrapped)
    }
    for (item in items) {
        // Collections.synchronizedList is not used here.
        consume("Collections.synchronizedMap $item")
    }
    for (item in items) {
        val wrapped = Collections.synchronizedMap(mutableMapOf(item to 1))
        wrapped[item] = 2
        consume(wrapped.getValue(item) + wrapped.size)
        consume(Collections.synchronizedList(mutableListOf(item)).also { consume(it) }.toString())
    }
    // Map.forEach with a two-parameter lambda is the Java member, which
    // java.util.Collections runs under the wrapper's lock.
    for (item in items) {
        Collections.synchronizedMap(mutableMapOf(item to 1)).forEach { key, value -> consume(key + value) }
    }
}

// The other lock does not protect a newly created wrapper's iterator.
fun bodyIterationUnderLock(items: List<Int>, lists: List<MutableList<Int>>, lock: Any) {
    <!CollectionsSynchronizedListIteration!>for<!> (item in items) {
        synchronized(lock) {
            Collections.synchronizedList(mutableListOf(item)).forEach { consume(it) }
        }
    }
    for (list in lists) {
        synchronized(lock) {
            <!CollectionsSynchronizedListIteration!>for<!> (item in Collections.synchronizedList(list)) consume(item)
        }
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
// correct to drop them because the loop iterates indices, other elements, a
// snapshot, a String, or one element's value, and the wrapper is only read
// through a member that java.util.Collections runs under the wrapper's own
// lock (size, contains, get, toArray, toString, a view's size) or a stdlib
// call built only on such members (first() of a List, indices, getValue,
// toTypedArray, and toList() or sorted() of a List): no iterator of the
// wrapper is used.
fun guardedReads(list: MutableList<Int>, map: MutableMap<String, List<Int>>, nested: MutableList<List<Int>>) {
    for (index in 0 until Collections.synchronizedList(list).size) consume(index)
    for (item in list.filter { Collections.synchronizedList(list).contains(it) }) consume(item)
    for (char in Collections.synchronizedList(list).first().toString()) consume(char)
    for (item in Collections.synchronizedList(list).toTypedArray()) consume(item)
    for (item in Collections.synchronizedList(list).toList()) consume(item)
    for (item in Collections.synchronizedList(list).sorted()) consume(item)
    for (index in Collections.synchronizedList(list).indices) consume(index)
    for (item in Collections.synchronizedMap(map)["k"]!!) consume(item)
    for (item in Collections.synchronizedMap(map).getValue("k")) consume(item)
    for (item in Collections.synchronizedMap(map)["k"].orEmpty()) consume(item)
    for (item in Collections.synchronizedList(nested).first()) consume(item)
    for (char in Collections.synchronizedList(list).toString()) consume(char)
    for (index in 0 until Collections.synchronizedMap(map).keys.size) consume(index)
}

// Go misses these because the header text splits the qualifier from the
// call (a line break or a comment between `Collections` and
// `.synchronizedList`), so its substring match fails; FIR reports them
// because the loop iterates the wrapper.
fun splitQualifier(nums: MutableList<Int>) {
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections
        .synchronizedList(nums)) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Collections /* shared */ .synchronizedList(nums)) consume(item)
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
