// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17
// A project class named Collections is not java.util.Collections, and a
// local variable named like a wrapper holds a plain list.
package test

object Collections {
    fun <T> synchronizedList(list: MutableList<T>): MutableList<T> = list
    fun <T> synchronizedSet(set: MutableSet<T>): MutableSet<T> = set
}

// Go reports these because the header spells Collections.synchronizedList;
// FIR is correct to drop them because the project's Collections returns the
// list it was given, not a synchronized wrapper.
fun lookalikes() {
    for (item in Collections.synchronizedList(mutableListOf(1))) consume(item)
    for (item in Collections.synchronizedSet(mutableSetOf(1))) consume(item)
    val synchronizedList = Collections.synchronizedList(mutableListOf(1))
    for (item in synchronizedList) consume(item)
}

private fun consume(value: Any?) {
    println(value)
}
