// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (recall): a user typealias to HashMap. Each call constructs a
// java.util.HashMap with an Int key, so FIR reports it ("Use SparseArray
// instead of HashMap<Int, ...>"). Go misses both, because the call name is the
// alias (`IntMap`, `JMap`), not HashMap.
package test

typealias IntMap<V> = HashMap<Int, V>
typealias JMap<K, V> = java.util.HashMap<K, V>

fun intMap(): Map<Int, String> = <!UseSparseArrays!>IntMap<String>()<!>

fun jMap(): Map<Int, String> = <!UseSparseArrays!>JMap<Int, String>()<!>

// An alias whose key is not Int or Long is reported by neither.
typealias NameMap<V> = HashMap<String, V>

fun nameMap(): Map<String, String> = NameMap<String>()
