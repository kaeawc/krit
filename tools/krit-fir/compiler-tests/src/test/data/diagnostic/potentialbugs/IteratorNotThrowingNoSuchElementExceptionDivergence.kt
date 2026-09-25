// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 31, 38, 44, 53, 60, 75, 88, 101, 114, 126, 134, 145, 158
// Where FIR resolution and the Go rule's name-based matching disagree. Go
// takes any function named `next` whose nearest enclosing class declaration
// mentions Iterator in a supertype list anywhere in its body, and reads the
// thrown exception by the name of the call inside `throw`. FIR reports only
// an iterator's own next(), and reads what the body really throws.
package test

import java.util.NoSuchElementException as JavaNoSuchElement

class MissingElement : NoSuchElementException()

// --- Go findings FIR drops: not an iterator's next() ---

class WithLocalNext(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    override fun next(): Int {
        // Go reports this because the enclosing class is an iterator; FIR is
        // correct to drop it because a local function is not its next().
        fun next(): Int = items[index]
        if (!hasNext()) throw NoSuchElementException()
        return next().also { index++ }
    }

    // Go reports this because the name is next; FIR is correct to drop it
    // because an overload with parameters is not Iterator.next().
    fun next(step: Int): Int {
        index += step
        return items[index - 1]
    }

    // Go reports this because the name is next; FIR is correct to drop it
    // because an extension is not Iterator.next().
    fun String.next(): Char = this[index]

    companion object {
        // Go reports this because tree-sitter skips the companion and finds the
        // iterator class; FIR is correct to drop it because the companion is
        // not an iterator.
        fun next(): Int = 0
    }

    val helper = object : Runnable {
        override fun run() {}

        // Go reports this because it skips the anonymous object and finds the
        // iterator class; FIR is correct to drop it because the anonymous
        // object is not an iterator.
        fun next(): Int = 0
    }
}

class Container {
    // Go reports this because the class body contains an iterator's supertype
    // list; FIR is correct to drop it because Container is not an iterator.
    fun next(): Int = 0

    class Inner : Iterator<Int> {
        override fun hasNext(): Boolean = false

        override fun next(): Int = throw NoSuchElementException()
    }
}

class ComparesIterators : Comparable<Iterator<Int>> {
    override fun compareTo(other: Iterator<Int>): Int = 0

    // Go reports this because `Iterator` appears as a type argument in the
    // supertype list; FIR is correct to drop it because the class is not an
    // iterator.
    fun next(): Int = 0
}

// --- Go findings FIR drops: the body does throw NoSuchElementException ---

class ThrowsStored(private val items: List<Int>) : Iterator<Int> {
    private var index = 0
    private val exhausted = NoSuchElementException("exhausted")

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because no call named NoSuchElementException is thrown;
    // FIR is correct to drop it because `exhausted` is one.
    override fun next(): Int {
        if (!hasNext()) throw exhausted
        return items[index++]
    }
}

class ThrowsSubclass(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because the call is named MissingElement; FIR is correct
    // to drop it because MissingElement is a NoSuchElementException.
    override fun next(): Int {
        if (!hasNext()) throw MissingElement()
        return items[index++]
    }
}

class ThrowsImportAlias(private val items: List<Int>) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because the call is named JavaNoSuchElement; FIR is
    // correct to drop it because it constructs java.util.NoSuchElementException.
    override fun next(): Int {
        if (!hasNext()) throw JavaNoSuchElement()
        return items[index++]
    }
}

class ThrowsAnonymousSubclass : Iterator<Int> {
    override fun hasNext(): Boolean = false

    // Go reports this because an object expression is not a call named
    // NoSuchElementException; FIR is correct to drop it because the anonymous
    // object is a NoSuchElementException.
    override fun next(): Int = throw object : NoSuchElementException("exhausted") {}
}

class ThrowsLocalSubclass : Iterator<Int> {
    override fun hasNext(): Boolean = false

    // Go reports this because the call is named Done; FIR is correct to drop
    // it because the local class Done is a NoSuchElementException.
    override fun next(): Int {
        class Done : NoSuchElementException()
        throw Done()
    }
}

class ThrowsReflective : Iterator<Int> {
    override fun hasNext(): Boolean = false

    // Go reports this because the call is named newInstance; FIR is correct to
    // drop it because the instance is a NoSuchElementException.
    override fun next(): Int {
        throw NoSuchElementException::class.java.getDeclaredConstructor().newInstance()
    }
}

class ThrowsSubclassInBranch(private val items: List<Int>, private val strict: Boolean) : Iterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size

    // Go reports this because no call named NoSuchElementException is thrown;
    // FIR is correct to drop it because the strict branch throws a
    // NoSuchElementException subclass.
    override fun next(): Int {
        if (!hasNext()) throw if (strict) MissingElement() else IllegalStateException()
        return items[index++]
    }
}

// --- True positives Go misses ---

// Go misses this because an anonymous object outside any class declaration
// has no enclosing class for it to inspect.
fun topLevelAnonymous(): Iterator<Int> = object : Iterator<Int> {
    override fun hasNext(): Boolean = false

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
}

// Go misses this because an anonymous object in a top-level property
// initializer has no enclosing class for it to inspect.
val topProp: Iterator<Int> = object : Iterator<Int> {
    override fun hasNext(): Boolean = false

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
}

// Go misses this because MutableListIterator is not in its name list.
class MutableListed(private val items: MutableList<Int>) : MutableListIterator<Int> {
    private var index = 0

    override fun hasNext(): Boolean = index < items.size
    override fun hasPrevious(): Boolean = index > 0
    override fun nextIndex(): Int = index
    override fun previousIndex(): Int = index - 1
    override fun previous(): Int = items[--index]
    override fun remove() {}
    override fun set(element: Int) {}
    override fun add(element: Int) {}

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = items[index++]
}

// Go misses this because the supertype's name does not show it is an iterator.
interface IntSource : Iterator<Int>

class FromSource : IntSource {
    override fun hasNext(): Boolean = false

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
}

abstract class BaseIterator<T> : Iterator<T>

class FromBase : BaseIterator<String>() {
    override fun hasNext(): Boolean = false

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): String = ""
}

// Go misses this because the open base class's name does not show it is an
// iterator; a concrete open class counts, not only an interface or an
// abstract class.
open class OpenIter : Iterator<Int> {
    override fun hasNext(): Boolean = false

    override fun next(): Int = throw NoSuchElementException()
}

class SubIter : OpenIter() {
    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 1
}

typealias Ints = Iterator<Int>

// Go misses this because the type alias's name is not an iterator name.
class FromAlias : Ints {
    override fun hasNext(): Boolean = false

    <!IteratorNotThrowingNoSuchElementException!>override<!> fun next(): Int = 0
}
