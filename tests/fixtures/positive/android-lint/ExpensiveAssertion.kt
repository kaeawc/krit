package com.example

class Cache(private val entries: MutableList<String>) {
    private var hits = 0

    fun verify() {
        assert(computeChecksum() == 0)
        assert(listOf(1, 2).size == hits)
        assert(if (hits > 0) true else false)
        assert(Entry("a") != null)
    }

    private fun computeChecksum(): Int {
        var sum = 0
        entries.forEach { sum += it.length }
        return sum
    }
}

class Entry(val key: String)

fun topLevel(count: Int) {
    assert(maxOf(count, 1) > 0)
}
