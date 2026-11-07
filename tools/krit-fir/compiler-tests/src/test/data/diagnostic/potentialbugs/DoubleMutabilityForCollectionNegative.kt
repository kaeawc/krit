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
    private var readOnly: List<String> = emptyList()

    private var readOnlyInferred = listOf("a")

    private var readOnlyMap: Map<String, Int> = emptyMap()

    // A wrapper whose name merely contains a mutable collection's name.
    private var wrapper: MyMutableListWrapper = MyMutableListWrapper()

    // A mutable collection nested as a type argument.
    private var nested: List<MutableList<String>> = emptyList()

    // Mutable types the default mutableTypes do not list, not built by a
    // listed factory.
    private var concurrent: ConcurrentHashMap<String, Int> = ConcurrentHashMap()

    private var collection: MutableCollection<String> = build()

    private var deque = ArrayDeque<String>()

    // Text in a string or a comment is not an initializer.
    private var text = "mutableListOf<String>()"

    /**
     * mutableListOf<String>() is mentioned here.
     */
    private var documented = emptyList<String>()

    // A function type returning a mutable list.
    private var supplier: () -> MutableList<String> = { mutableListOf() }

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

// A Java method returning java.util.List / Set / Map has the platform type
// `(Mutable)List<T>!`, whose read-only upper bound says the collection may not
// be mutable (an immutable Collections.emptyList(), an unmodifiable
// System.getenv()). Go reports none of these either.
class PlatformTypes {
    private var keys = java.util.Collections.emptyList<String>()

    private var env = System.getenv()

    private var flexList = java.util.Arrays.asList("a", "b")

    private var threads = Thread.getAllStackTraces()

    private var names = System.getProperties().stringPropertyNames()
}

// Vars that have to stay `var`: a declared nullable lazy-init holder, a var
// written after its declaration (by name, also through `this`), and members
// other files can write. Go drops these too.
class MustStayVar {
    private var lazy: MutableList<String>? = null

    private var reassigned: MutableList<String> = mutableListOf()

    private var viaThis: MutableMap<String, Int> = mutableMapOf()

    var visible: MutableList<String> = mutableListOf()

    internal var internalList: MutableSet<String> = mutableSetOf()

    fun reset() {
        reassigned = mutableListOf()
        this.viaThis = mutableMapOf()
    }
}

interface Api {
    var abstractList: MutableList<String>
}

class Impl : Api {
    override var abstractList: MutableList<String> = mutableListOf()
}

var topLevelVisible: MutableList<String> = mutableListOf()

fun localReassigned(): Int {
    var list = mutableListOf<String>()
    var set: MutableSet<String> = mutableSetOf()
    list = mutableListOf()
    set = mutableSetOf()
    return list.size + set.size
}
