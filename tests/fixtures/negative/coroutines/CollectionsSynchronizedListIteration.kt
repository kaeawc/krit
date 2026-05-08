package test

import java.util.Collections
import java.util.concurrent.CopyOnWriteArrayList

fun iterateWithExternalSynchronization() {
    val list = Collections.synchronizedList(mutableListOf(1, 2, 3))
    synchronized(list) {
        for (item in list) {
            consume(item)
        }
    }
}

fun iterateCopyOnWriteList() {
    val copyOnWrite = CopyOnWriteArrayList(listOf(1, 2, 3))
    for (item in copyOnWrite) {
        consume(item)
    }
}

private fun consume(value: Any?) {
    println(value)
}
