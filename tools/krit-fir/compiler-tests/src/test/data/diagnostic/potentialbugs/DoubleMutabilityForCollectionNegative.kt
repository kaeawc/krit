// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: not a `var` with a mutable collection type. Go reports none of
// these either.
package test

import java.util.concurrent.ConcurrentHashMap

class MyMutableListWrapper

class Negative {
    // `val` with a mutable collection: the recommended shape.
    val list: MutableList<String> = mutableListOf()

    val inferred = mutableListOf<String>()

    // `var` with a read-only collection: the other recommended shape.
    var readOnly: List<String> = emptyList()

    var readOnlyInferred = listOf("a")

    var readOnlyMap: Map<String, Int> = emptyMap()

    // A wrapper whose name merely contains a mutable collection's name.
    var wrapper: MyMutableListWrapper = MyMutableListWrapper()

    // A mutable collection nested as a type argument.
    var nested: List<MutableList<String>> = emptyList()

    // Mutable types the default mutableTypes do not list, not built by a
    // listed factory.
    var concurrent: ConcurrentHashMap<String, Int> = ConcurrentHashMap()

    var collection: MutableCollection<String> = build()

    var deque = ArrayDeque<String>()

    // Text in a string or a comment is not an initializer.
    var text = "mutableListOf<String>()"

    /**
     * mutableListOf<String>() is mentioned here.
     */
    var documented = emptyList<String>()

    // A function type returning a mutable list.
    var supplier: () -> MutableList<String> = { mutableListOf() }

    fun build(): MutableCollection<String> = mutableListOf()
}

// A primary-constructor `val` is not a `var`.
class ConstructorVal(val items: MutableList<String>)

fun destructuring(): Int {
    var (first, second) = Pair(mutableListOf<String>(), mutableSetOf<String>())
    first = mutableListOf()
    second = mutableSetOf()
    return first.size + second.size
}

fun loop(lists: List<MutableList<String>>): Int {
    var total = 0
    for (list in lists) total += list.size
    return total
}
