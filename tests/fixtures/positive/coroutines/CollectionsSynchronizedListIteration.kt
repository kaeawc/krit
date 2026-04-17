package test

import java.util.Collections

fun iterateSynchronizedWrappers() {
    for (item in Collections.synchronizedList(mutableListOf(1, 2, 3))) {
        consume(item)
    }

    for (item in Collections.synchronizedSet(mutableSetOf("a", "b"))) {
        consume(item)
    }

    for ((key, value) in Collections.synchronizedMap(mutableMapOf("a" to 1))) {
        consume(key)
        consume(value)
    }
}

private fun consume(value: Any?) {
    println(value)
}
