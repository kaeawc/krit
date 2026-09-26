// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses these because the loop header does not spell
// `Collections.synchronized...`; FIR reports them because the call resolves
// to the java.util.Collections wrapper factory.
package test

import java.util.Collections as Colls
import java.util.Collections.synchronizedList
import java.util.Collections.synchronizedMap as syncMap

fun imported(list: MutableList<Int>, map: MutableMap<String, Int>) {
    <!CollectionsSynchronizedListIteration!>for<!> (item in synchronizedList(list)) consume(item)
    <!CollectionsSynchronizedListIteration!>for<!> ((key, value) in syncMap(map)) consume(key + value)
    <!CollectionsSynchronizedListIteration!>for<!> (item in Colls.synchronizedSet(mutableSetOf(1))) consume(item)
}

private fun consume(value: Any?) {
    println(value)
}
