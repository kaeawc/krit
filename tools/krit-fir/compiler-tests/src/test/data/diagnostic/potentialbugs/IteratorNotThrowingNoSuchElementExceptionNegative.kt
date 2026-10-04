// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: iterators whose next() throws NoSuchElementException somewhere in
// its body, and next() functions that do not belong to an iterator. The Go
// rule reports none of these either.
package test

class Guarded<T>(private val items: List<T>) : Iterator<T> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): T {
        if (!hasNext()) throw NoSuchElementException()
        return items[index++]
    }
}

class GuardedWithMessage(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException("index $index")
        return items[index++]
    }
}

// Expression body with elvis.
class Elvis(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int = items.getOrNull(index++) ?: throw NoSuchElementException()
}

// The Java class, qualified.
class Qualified(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int {
        if (!hasNext()) throw java.util.NoSuchElementException()
        return items[index++]
    }
}

// Inside a lambda in the body; Go walks the whole body.
class InLambda(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int = items.getOrElse(index++) { throw NoSuchElementException() }
}

// Inside a when branch.
class InWhen(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int = when {
        index < items.size -> items[index++]
        else -> throw NoSuchElementException()
    }
}

// The thrown expression constructs one in a branch.
class ChosenException(private val items: List<Int>, private val strict: Boolean) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int {
        if (!hasNext()) throw if (strict) NoSuchElementException() else IllegalStateException()
        return items[index++]
    }
}

class MutableGuarded(private val items: MutableList<Int>) : MutableIterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int {
        if (!hasNext()) throw NoSuchElementException()
        return items[index++]
    }

    override fun remove() {
        items.removeAt(--index)
    }
}

object Empty : Iterator<Nothing> {
    override fun hasNext(): Boolean = false

    override fun next(): Nothing = throw NoSuchElementException()
}

fun anonymousGuarded(): Iterator<Int> = object : Iterator<Int> {
    override fun hasNext(): Boolean = false

    override fun next(): Int = throw NoSuchElementException()
}

// Abstract next() has no body.
abstract class AbstractNext : Iterator<Int> {
    abstract override fun next(): Int
}

// Not an iterator: a class with its own next().
class Sequence {
    private var value = 0

    fun next(): Int = value++
}

// Not an iterator: Iterable is not Iterator.
class Bag(private val items: List<Int>) : Iterable<Int> {
    override fun iterator(): Iterator<Int> = items.iterator()

    fun next(): Int = items.first()
}

// Top-level next() has no enclosing class.
fun next(): Int = 0
