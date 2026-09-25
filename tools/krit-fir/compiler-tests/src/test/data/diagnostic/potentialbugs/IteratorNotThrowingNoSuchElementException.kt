// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: an iterator's next() whose body never throws
// NoSuchElementException. Each is reported on the function's first line
// (its modifier list, else `fun`), the line the Go rule reports.
package test

class ListBacked<T>(private val items: List<T>) : Iterator<T> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): T {
        return items[index++]
    }
}

class ExpressionBody(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]
}

class Annotated(private val items: List<String>) : kotlin.collections.Iterator<String> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    /**
     * KDoc is not part of the reported line.
     */
    <!IteratorNotThrowingNoSuchElementException!>@Suppress("UNUSED")<!>
    override fun next(): String = items[index++]
}

class Mutable(private val items: MutableList<Int>) : MutableIterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]

    override fun remove() {
        items.removeAt(--index)
    }
}

class Listed(private val items: List<Int>) : ListIterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size
    override fun hasPrevious(): Boolean = index > 0
    override fun nextIndex(): Int = index
    override fun previousIndex(): Int = index - 1
    override fun previous(): Int = items[--index]

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]
}

object Counter : Iterator<Int> {
    private var count = 0

    override fun hasNext(): Boolean = true

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = count++
}

// An interface default next() is the iterator's next() too.
interface DefaultNext : Iterator<Int> {
    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int {
        return 0
    }
}

// Delegating to another iterator is not a throw in the body; Go reports it too.
class Delegating(private val inner: Iterator<Int>) : Iterator<Int> {
    override fun hasNext(): Boolean = inner.hasNext()

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = inner.next()
}

// A helper that throws is not a throw in the body; Go reports it too.
class HelperThrows(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    private fun ensureNext() {
        if (!hasNext()) throw NoSuchElementException()
    }

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int {
        ensureNext()
        return items[index++]
    }
}

// Throwing a different exception is not enough.
class WrongException(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int {
        if (!hasNext()) throw IllegalStateException("exhausted")
        return items[index++]
    }
}

class Outer {
    // A nested iterator class.
    class Nested : Iterator<Int> {
        override fun hasNext(): Boolean = false

        <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
    }

    // An anonymous iterator inside a class; Go finds it through the class.
    fun iterator(): Iterator<Int> = object : Iterator<Int> {
        override fun hasNext(): Boolean = false

        <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
    }
}

fun localIterator(): Iterator<Int> {
    // A local iterator class.
    class Local : Iterator<Int> {
        override fun hasNext(): Boolean = false

        <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
    }
    return Local()
}

enum class Direction : Iterator<Direction> {
    UP {
        override fun hasNext(): Boolean = true

        <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Direction = DOWN
    },
    DOWN {
        override fun hasNext(): Boolean = true

        <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Direction = UP
    },
}

// A throw that is not a NoSuchElementException does not count, even when the
// exception mentions one in a string.
class MessageOnly(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int {
        if (!hasNext()) throw IllegalStateException("NoSuchElementException")
        return items[index++]
    }
}

// Java interop: the platform iterator named directly.
@Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
class PlatformIterator(private val items: List<Int>) : java.util.Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]
}
