// RENDER_DIAGNOSTICS_FULL_TEXT
// Local lookalike: this package declares its own Iterator, which shadows
// kotlin.collections.Iterator in this file. Its next() has no
// NoSuchElementException contract, so neither Go (which skips an Iterator
// declared in the same file) nor FIR reports it.
package lookalike

interface Iterator<T> {
    fun hasNext(): Boolean
    fun next(): T
}

class Cursor(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int = items[index++]
}

// The real iterator, qualified, is still an iterator. Go misses this because
// the supertype's simple name matches the Iterator declared in this file; FIR
// resolves it to kotlin.collections.Iterator.
class RealCursor(private val items: List<Int>) : kotlin.collections.Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]
}
