// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: a declaration in the file named like a built-in
// collection makes Go treat the name as shadowed, but these declarations are
// mutable collections themselves (a java.util.LinkedHashMap subclass, a type
// alias of CopyOnWriteArraySet), so the property's type is still a mutable
// collection. The same declarations in another file of the package are Go
// findings (DoubleMutabilityForCollectionCrossFileTest).
package test

class LinkedHashMap<K, V> : java.util.LinkedHashMap<K, V>()

typealias HashSet<T> = java.util.concurrent.CopyOnWriteArraySet<T>

fun <K, V> cache(): LinkedHashMap<K, V> = LinkedHashMap()

fun <T> fresh(): HashSet<T> = HashSet()

class Holder {
    <!DoubleMutabilityForCollection!>var<!> map: LinkedHashMap<String, Int> = cache()

    <!DoubleMutabilityForCollection!>var<!> set: HashSet<String> = fresh()
}
