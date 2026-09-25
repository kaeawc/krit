// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: an iterator's hasNext() whose body calls next(). Each is reported
// once, on the function's first line (its modifier list, else `fun`), the line
// the Go rule reports.
package test

import java.sql.ResultSet
import java.util.Scanner

class Delegating<T>(private val items: Iterator<T>) : Iterator<T> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        return items.next() != null
    }

    override fun next(): T = items.next()
}

class OwnNext(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        next()
        return index < items.size
    }

    override fun next(): Int = items[index++]
}

class ExpressionBody(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = this.next() >= 0

    override fun next(): Int = items[index++]
}

class Annotated(private val items: Iterator<String>) : kotlin.collections.Iterator<String> {
    /**
     * KDoc is not part of the reported line.
     */
    <!IteratorHasNextCallsNextMethod!>@Suppress("UNUSED")<!>
    override fun hasNext(): Boolean = items.next().isNotEmpty()

    override fun next(): String = items.next()
}

class TwoCalls(private val items: Iterator<Int>) : Iterator<Int> {
    // One finding per function, however many next() calls it makes.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        val first = items.next()
        val second = items.next()
        return first < second
    }

    override fun next(): Int = items.next()
}

class SafeCall(private val items: Iterator<Int>?) : Iterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items?.next() != null

    override fun next(): Int = items!!.next()
}

class InLambda(private val items: Iterator<Int>) : Iterator<Int> {
    // Go walks the whole body, into lambdas.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.let { it.next() > 0 }

    override fun next(): Int = items.next()
}

class WithReceiver(private val items: Iterator<Int>) : Iterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = with(items) { next() > 0 }

    override fun next(): Int = items.next()
}

class InLocalFunction(private val items: Iterator<Int>) : Iterator<Int> {
    // Go walks the whole body, into local functions.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        fun peek(): Int = items.next()
        return peek() > 0
    }

    override fun next(): Int = items.next()
}

class Mutable(private val items: MutableIterator<Int>) : MutableIterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()

    override fun remove() = items.remove()
}

class Listed(private val items: ListIterator<Int>) : ListIterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0
    override fun hasPrevious(): Boolean = items.hasPrevious()
    override fun nextIndex(): Int = items.nextIndex()
    override fun previousIndex(): Int = items.previousIndex()
    override fun previous(): Int = items.previous()
    override fun next(): Int = items.next()
}

class JavaIterator(private val items: java.util.Iterator<Int>) : java.util.Iterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()
}

class Tokens(private val scanner: Scanner) : Iterator<String> {
    // Scanner is a java.util.Iterator, so its next() advances it.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = scanner.next().isNotEmpty()

    override fun next(): String = scanner.next()
}

class FromJavaCollection(private val items: java.util.ArrayList<String>) : Iterator<String> {
    private val source = items.iterator()

    // The iterator of a Java collection is a platform type.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = source.next() != null

    override fun next(): String = source.next()
}

object Singleton : Iterator<Int> {
    private val items = listOf(1, 2, 3).iterator()

    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()
}

interface Peeking : Iterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = next() > 0
}

class Stepping(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // An overload of next on the iterator advances it too; Go reports any call
    // named next.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = next(0) >= 0

    override fun next(): Int = items[index++]

    fun next(step: Int): Int {
        index += step
        return items[index]
    }
}

fun <T> Iterator<T>.next(count: Int): List<T> = List(count) { next() }

class ExtensionNext(private val items: Iterator<Int>) : Iterator<Int> {
    // An extension named next on an iterator advances it; Go reports any call
    // named next.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next(1).isNotEmpty()

    override fun next(): Int = items.next()
}

class Outer {
    // Go reports the anonymous iterator inside a class: the class body
    // contains its Iterator supertype list.
    val cursor = object : Iterator<Int> {
        private val items = listOf(1).iterator()

        <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

        override fun next(): Int = items.next()
    }

    class Nested(private val items: Iterator<Int>) : Iterator<Int> {
        <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

        override fun next(): Int = items.next()
    }

    companion object Cursor : Iterator<Int> {
        private val items = listOf(1).iterator()

        <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

        override fun next(): Int = items.next()
    }
}

fun localIterator(items: Iterator<Int>): Iterator<Int> {
    class Local : Iterator<Int> {
        <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

        override fun next(): Int = items.next()
    }
    return Local()
}

// --- Any call named next counts, like Go, whatever declares it ---

class Rows(private val rows: ResultSet) : Iterator<String> {
    // ResultSet is not an Iterator, but next() advances its cursor: the classic
    // JDBC iterator bug.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = rows.next()

    override fun next(): String = rows.getString(1)
}

class Node(val value: Int) {
    fun next(): Node? = null
}

class Linked(private var node: Node?) : Iterator<Int> {
    // Resolution cannot tell a side-effect-free next() from a cursor's, so a
    // call named next counts, as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = node?.next() != null || node != null

    override fun next(): Int {
        val current = node ?: throw NoSuchElementException()
        node = current.next()
        return current.value
    }
}

class LocalNext(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // A local function named next, as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        fun next(): Int = index
        return next() < items.size
    }

    override fun next(): Int = items[index++]
}

class FunctionTyped(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // The invocation of a function-typed local value named next, as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        val next: () -> Int = { index + 1 }
        return next() <= items.size
    }

    override fun next(): Int = items[index++]
}

class Stepper(val next: () -> Int)

class FunctionTypedProperty(private val stepper: Stepper, private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // The invocation of a function-typed property named next, as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = stepper.next() <= items.size

    override fun next(): Int = items[index++]
}

enum class Countdown : Iterator<Int> {
    ONCE {
        // An enum entry's member: the entry is a Countdown, an iterator.
        <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = next() > 0
    },
    NEVER;

    override fun hasNext(): Boolean = false

    override fun next(): Int = 0
}

class NextInLoopBody(private val items: Iterator<Int>, private val limits: List<Int>) : Iterator<Int> {
    // The for-loop's own generated next() does not count, but a real next()
    // call inside its body does.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        for (limit in limits) {
            if (items.next() > limit) return true
        }
        return false
    }

    override fun next(): Int = items.next()
}

class NextInLoopRange(private val items: Iterator<Int>) : Iterator<Int> {
    // A real next() call in a for-loop's range expression.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        for (x in 0 until items.next()) {
            if (x > 0) return true
        }
        return false
    }

    override fun next(): Int = items.next()
}
