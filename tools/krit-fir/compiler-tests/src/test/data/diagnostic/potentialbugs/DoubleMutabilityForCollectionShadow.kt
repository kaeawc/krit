// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 33, 42
// Same-named declarations. Go treats a built-in collection name as shadowed in
// the whole file once any class, object, interface, or type alias in the file
// declares it, wherever it is nested, and falls back to the initializer's call
// name.
package test

import java.util.concurrent.ConcurrentHashMap as HashMap

fun <T> build(): MutableList<T> = mutableListOf()

class Scope {
    class MutableList

    class LinkedHashSet

    // A lookalike: Scope.MutableList is not a collection.
    var nested: MutableList = MutableList()

    // Go reports this through its factory-name fallback (the call is spelled
    // `LinkedHashSet()`), but Scope.LinkedHashSet is not a collection.
    var nestedSet: LinkedHashSet = LinkedHashSet()
}

// Positive Go misses: Scope.MutableList is not in scope here, so this is
// kotlin.collections.MutableList, but Go sees the nested declaration's name.
<!DoubleMutabilityForCollection!>var<!> topLevel: MutableList<String> = build()

// Positive like Go: `HashMap` is the import alias of ConcurrentHashMap, a
// mutable collection. Go ignores the aliased type name and reports through the
// factory-name fallback, since the call is spelled `HashMap(...)`.
<!DoubleMutabilityForCollection!>var<!> concurrent: HashMap<String, Int> = HashMap()

// Negative like Go: the same aliased type without a factory-named call.
var concurrentBuilt: HashMap<String, Int> = HashMap<String, Int>().also { it.clear() }

fun local(): Int {
    class ArrayList : java.util.ArrayList<String>()

    // Positive like Go: the local ArrayList is a java.util.ArrayList subclass.
    <!DoubleMutabilityForCollection!>var<!> local = ArrayList()
    local = ArrayList()
    return local.size
}
