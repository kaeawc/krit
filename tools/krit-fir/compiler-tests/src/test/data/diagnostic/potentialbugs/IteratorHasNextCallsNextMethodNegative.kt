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
