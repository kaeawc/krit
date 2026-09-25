// RENDER_DIAGNOSTICS_FULL_TEXT
// Calls written `next(...)` that do not resolve to a function declared as
// next: the invocation of an object named next, the constructor of a class
// named next, and a function imported under the alias next. Go reports any
// call whose name is written next, and FIR matches it. A function declared as
// next but imported under another name is not written next, so neither
// reports it.
package test.namednext

import test.namednext.Steps.advance as next
import test.namednext.Steps.next as step

object next {
    operator fun invoke(): Int = 0
}

class Holder {
    class next(val value: Int)
}

object Steps {
    fun advance(step: Int, extra: Int): Int = step + extra

    fun next(): Int = 0
}

class Qualified(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // The object named next, invoked through its qualified name, as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = test.namednext.next() < items.size

    override fun next(): Int = items[index++]
}

class Constructs(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // The constructor of a class named next, as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = Holder.next(index).value < items.size

    override fun next(): Int = items[index++]
}

class AliasedToNext(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // Steps.advance() imported as next is called as next(), as in Go.
    <!IteratorHasNextCallsNextMethod!>override<!> fun hasNext(): Boolean = next(index, 1) < items.size

    override fun next(): Int = items[index++]
}

class AliasedAway(private val items: IntArray) : Iterator<Int> {
    private var index = 0

    // Steps.next() imported as step is not written next: neither reports.
    override fun hasNext(): Boolean = step() + index < items.size

    override fun next(): Int = items[index++]
}
