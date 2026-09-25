// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (precision): a same-package function named HashMap wins over the
// default-imported HashMap class, so the unqualified call is not a
// java.util.HashMap constructor call. Go reports it because it matches the
// call name alone. The qualified call still constructs the JDK map.
package test

fun <K, V> HashMap(): MutableMap<K, V> = mutableMapOf()

fun lookalike(): MutableMap<Int, String> = HashMap<Int, String>()

fun jdk(): Map<Int, String> = <!UseSparseArrays!>kotlin.collections.HashMap<Int, String>()<!>
