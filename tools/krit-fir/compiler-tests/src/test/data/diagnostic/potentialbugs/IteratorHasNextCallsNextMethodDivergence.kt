// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 26, 30, 38, 47, 54, 69
// Where FIR resolution and the Go rule's name-based matching disagree. Go
// takes any function named `hasNext` whose nearest enclosing class declaration
// mentions Iterator in a supertype list anywhere in its body. FIR reports only
// an iterator's own hasNext(). Both take any call named `next` in its body.
package test

import kotlin.collections.MutableIterator as Cursor

// --- Go findings FIR drops: not an iterator's hasNext() ---

class WithLocalHasNext(private val items: Iterator<Int>) : Iterator<Int> {
    override fun hasNext(): Boolean = items.hasNext()

    override fun next(): Int {
        // Go reports this because the enclosing class is an iterator; FIR is
        // correct to drop it because a local function is not its hasNext().
        fun hasNext(): Boolean = items.next() > 0
        hasNext()
        return 0
    }

    // Go reports this because the name is hasNext; FIR is correct to drop it
    // because an overload with parameters is not Iterator.hasNext().
    fun hasNext(step: Int): Boolean = items.next() > step

    // Go reports this because the name is hasNext; FIR is correct to drop it
    // because an extension is not Iterator.hasNext().
    fun String.hasNext(): Boolean = items.next() > length

    companion object {
        private val shared = listOf(1).iterator()

        // Go reports this because tree-sitter skips the companion and finds the
        // iterator class; FIR is correct to drop it because the companion is
        // not an iterator.
        fun hasNext(): Boolean = shared.next() > 0
    }

    val helper = object : Runnable {
        override fun run() {}

        // Go reports this because it skips the anonymous object and finds the
        // iterator class; FIR is correct to drop it because the anonymous
        // object is not an iterator.
        fun hasNext(): Boolean = items.next() > 0
    }
}

class Container(private val items: Iterator<Int>) {
    // Go reports this because the class body contains an iterator's supertype
    // list; FIR is correct to drop it because Container is not an iterator.
    fun hasNext(): Boolean = items.next() > 0

    class Inner : Iterator<Int> {
        override fun hasNext(): Boolean = false

        override fun next(): Int = throw NoSuchElementException()
    }
}

class ComparesIterators(private val items: Iterator<Int>) : Comparable<Iterator<Int>> {
    override fun compareTo(other: Iterator<Int>): Int = 0

    // Go reports this because `Iterator` appears as a type argument in the
    // supertype list; FIR is correct to drop it because the class is not an
    // iterator.
    fun hasNext(): Boolean = items.next() > 0
}

// A hasNext() with a context parameter (Go reports it; FIR drops it because it
// is not Iterator.hasNext()) needs -Xcontext-parameters to compile, so it is
// pinned in IteratorHasNextCallsNextMethodTest.contextParameterHasNext.

// --- True positives Go misses ---

// Go misses this because an anonymous object outside any class declaration
// has no enclosing class for it to inspect.
fun topLevelAnonymous(items: Iterator<Int>): Iterator<Int> = object : Iterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()
}

// Go misses this because an anonymous object in a top-level property
// initializer has no enclosing class for it to inspect.
val topProp: Iterator<Int> = object : Iterator<Int> {
    private val items = listOf(1).iterator()

    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()
}

// Go misses this because MutableListIterator is not in its name list.
class MutableListed(private val items: MutableListIterator<Int>) : MutableListIterator<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0
    override fun hasPrevious(): Boolean = items.hasPrevious()
    override fun nextIndex(): Int = items.nextIndex()
    override fun previousIndex(): Int = items.previousIndex()
    override fun previous(): Int = items.previous()
    override fun remove() = items.remove()
    override fun set(element: Int) = items.set(element)
    override fun add(element: Int) = items.add(element)
    override fun next(): Int = items.next()
}

// Go misses this because IntIterator is not in its name list.
class Ints(private val items: IntArray) : IntIterator() {
    private var index = 0

    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = next() >= 0

    override fun nextInt(): Int = items[index++]
}

// Go misses this because the supertype's name does not show it is an iterator.
interface IntSource : Iterator<Int>

class FromSource(private val items: Iterator<Int>) : IntSource {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()
}

abstract class BaseIterator<T> : Iterator<T>

class FromBase(private val items: Iterator<String>) : BaseIterator<String>() {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next().isNotEmpty()

    override fun next(): String = items.next()
}

typealias IntCursor = Iterator<Int>

// Go misses this because the type alias's name is not an iterator name.
class FromAlias(private val items: Iterator<Int>) : IntCursor {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()
}

// Go misses this because the import alias's name is not an iterator name.
class FromImportAlias(private val items: Cursor<Int>) : Cursor<Int> {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = items.next() > 0

    override fun next(): Int = items.next()

    override fun remove() = items.remove()
}

open class OpenIter(private val items: Iterator<Int>) : Iterator<Int> {
    override fun hasNext(): Boolean = items.hasNext()

    override fun next(): Int = items.next()
}

// Go misses this because the open base class's name does not show it is an
// iterator; super.next() is the base iterator's next.
class CallsSuper(items: Iterator<Int>) : OpenIter(items) {
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = super.next() > 0
}

class Parenthesized(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // Go misses this because it reads no call name through the parentheses;
    // it is the same invocation of a function-typed value named next that Go
    // reports without them (`FunctionTyped` in IteratorHasNextCallsNextMethod.kt).
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        val next: () -> Int = { index + 1 }
        return (next)() <= items.size
    }

    override fun next(): Int = items[index++]
}

class ReferenceInvoked(private val items: Iterator<Int>) : Iterator<Int> {
    // Go misses this because it reads no call name through the parentheses;
    // invoking the reference calls items.next().
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = (items::next)() > 0

    override fun next(): Int = items.next()
}

class Stride(val size: Int) {
    infix fun next(step: Int): Int = size + step
}

class InfixNext(private val stride: Stride, private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // Go misses this because tree-sitter parses the infix form as an
    // infix_expression, not a call_expression; it is the same call as
    // `stride.next(1)`, which Go reports (any call named next counts).
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = (stride next 1) <= items.size

    override fun next(): Int = items[index++]
}

class ForOverThis(private val values: IntArray) : Iterator<Int> {
    private var index = 0

    // Go misses this because the source has no call named next; a for-loop over
    // an iterator runs Iterator<T>.iterator(), which returns the iterator
    // itself, and then calls this.next() on each pass.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        for (v in this) {
            if (v < 0) return false
        }
        return index < values.size
    }

    override fun next(): Int = values[index++]
}

class ForOverField(private val items: Iterator<Int>) : Iterator<Int> {
    // Go misses this for the same reason; the loop calls items.next(), which
    // Go reports when it is written out.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        for (x in items) {
            if (x > 0) return true
        }
        return false
    }

    override fun next(): Int = items.next()
}

class ForOverScanner(private val scanner: java.util.Scanner) : Iterator<String> {
    // Scanner is a java.util.Iterator, so the loop calls scanner.next().
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean {
        for (token in scanner) {
            if (token.isNotEmpty()) return true
        }
        return false
    }

    override fun next(): String = scanner.next()
}
