// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: hasNext() implementations the Go rule and FIR both leave alone.
package test

class Indexed<T>(private val items: List<T>) : Iterator<T> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): T = items[index++]
}

class DelegatesHasNext(private val items: Iterator<Int>) : Iterator<Int> {
    override fun hasNext(): Boolean = items.hasNext()

    override fun next(): Int = items.next()
}

class ListedIndex(private val items: ListIterator<Int>) : ListIterator<Int> {
    // nextIndex() is not next().
    override fun hasNext(): Boolean = items.nextIndex() < 10
    override fun hasPrevious(): Boolean = items.hasPrevious()
    override fun nextIndex(): Int = items.nextIndex()
    override fun previousIndex(): Int = items.previousIndex()
    override fun previous(): Int = items.previous()
    override fun next(): Int = items.next()
}

class Reference(private val items: Iterator<Int>) : Iterator<Int> {
    // A callable reference is not a call.
    private val advance: () -> Int = items::next

    override fun hasNext(): Boolean {
        val unused = items::next
        return unused != advance
    }

    override fun next(): Int = advance()
}

abstract class Abstract : Iterator<Int> {
    // No body, nothing to inspect.
    abstract override fun hasNext(): Boolean
}

interface NoDefault : Iterator<Int> {
    override fun hasNext(): Boolean
}

class NotAnIterator(private val items: Iterator<Int>) {
    // Neither Go nor FIR: the class is not an iterator.
    fun hasNext(): Boolean = items.next() > 0
}

fun hasNext(items: Iterator<Int>): Boolean = items.next() > 0

class Iterator2(private val items: kotlin.collections.Iterator<Int>) {
    // A class named like an iterator is not one.
    fun hasNext(): Boolean = items.next() > 0
}

class Cursor : Iterator<Int> {
    private var remaining = 3

    // Calls another iterator's hasNext and this class's other members only.
    override fun hasNext(): Boolean = remaining > 0 && listOf(1).iterator().hasNext()

    override fun next(): Int = remaining--
}

// --- for-loops over collections, ranges, and maps ---
// FIR rewrites each for-loop into iterator()/hasNext()/next() calls on a fresh
// local iterator. That generated next() is not a call in the source, and it
// advances the loop's own iterator, not this one, so neither Go nor FIR
// reports it. (A for-loop over an iterator does advance it:
// `ForOverIterator` in IteratorHasNextCallsNextMethodDivergence.kt.)

class Flattening(private val lists: List<List<Int>>) : Iterator<Int> {
    private var outer = 0
    private var inner = 0

    // A for-loop over a list.
    override fun hasNext(): Boolean {
        for (l in lists) {
            if (l.isEmpty()) return false
        }
        return outer < lists.size
    }

    override fun next(): Int = lists[outer][inner++]
}

class RangeScan(private val lists: List<List<Int>>) : Iterator<Int> {
    private var outer = 0

    // A for-loop over an IntRange.
    override fun hasNext(): Boolean {
        for (i in outer until lists.size) {
            if (lists[i].isNotEmpty()) return true
        }
        return false
    }

    override fun next(): Int = lists[outer++].first()
}

class MapScan(private val map: Map<String, Int>) : Iterator<Int> {
    private var seen = 0

    // A for-loop over a Map, destructuring each entry.
    override fun hasNext(): Boolean {
        for ((k, v) in map) {
            if (k.isNotEmpty() && v > seen) return true
        }
        return false
    }

    override fun next(): Int = seen++
}

class LambdaScan(private val lists: List<List<Int>>) : Iterator<Int> {
    private var outer = 0

    // A for-loop inside a lambda.
    override fun hasNext(): Boolean = lists.any { l ->
        var any = false
        for (x in l) {
            if (x > outer) any = true
        }
        any
    }

    override fun next(): Int = outer++
}

class ArrayScan(private val values: IntArray) : Iterator<Int> {
    private var index = 0

    // A for-loop over an array and over a Sequence's iterator() result.
    override fun hasNext(): Boolean {
        for (v in values) {
            if (v < 0) return false
        }
        for (v in values.asSequence()) {
            if (v > 100) return false
        }
        return index < values.size
    }

    override fun next(): Int = values[index++]
}

class Reiterable(private val values: IntArray) : Iterator<Int>, Iterable<Int> {
    private var index = 0

    // `this` is an iterator here, but the loop calls the member iterator(),
    // which returns a fresh iterator, not this one.
    override fun hasNext(): Boolean {
        for (v in this) {
            if (v < 0) return false
        }
        return index < values.size
    }

    override fun next(): Int = values[index++]

    override fun iterator(): Iterator<Int> = values.iterator()
}

class FreshIteratorLoop(private val values: List<Int>) : Iterator<Int> {
    private var index = 0

    // A for-loop over an iterator a call returns advances that fresh
    // iterator, not this one, just like a loop over the list itself.
    override fun hasNext(): Boolean {
        for (v in values.iterator()) {
            if (v < 0) return false
        }
        return index < values.size
    }

    override fun next(): Int = values[index++]
}

class StepCalls(private val items: Iterator<Int>) : Iterator<Int> {
    private val steps = listOf(1)

    // A for-loop inside an iterator's hasNext() that only calls hasNext().
    override fun hasNext(): Boolean {
        for (s in steps) {
            if (!items.hasNext()) return false
        }
        return true
    }

    override fun next(): Int = items.next()
}
