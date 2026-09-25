// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (precision): a same-package class named HashMap is not
// java.util.HashMap. Go reports the unqualified call because it matches the
// call name alone, but the message ("Use SparseArray instead of
// HashMap<Int, ...>") is about the JDK map, which this code does not
// construct. The qualified JDK call still reports.
package test

class HashMap<K, V>

fun lookalike(): HashMap<Int, String> = HashMap<Int, String>()

fun jdk(): Map<Int, String> = <!UseSparseArrays!>java.util.HashMap<Int, String>()<!>
