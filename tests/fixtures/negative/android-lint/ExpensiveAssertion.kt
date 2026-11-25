package com.example

class Cache(private val entries: MutableList<String>) {
    private var hits = 0

    fun verify() {
        // Simple comparisons, qualified calls and cheap helpers are not reported.
        assert(hits > 0)
        assert(entries.isEmpty())
        assert(!entries.isNotEmpty() || hits == 0)
        assert(entries.size == hits)
        assert(isWarm())
        assert(this is Cache)
        assert(entries[hits] != null)
        if (javaClass.desiredAssertionStatus()) {
            assert(slowCheck())
        }
    }

    private fun isWarm(): Boolean = hits > 0

    private fun slowCheck(): Boolean {
        println("checking")
        return true
    }
}

class Custom {
    // A function of the file named assert is not kotlin.assert.
    fun assert(value: Boolean) = Unit

    fun check(x: Int) {
        assert(maxOf(x, 1) > 0)
    }
}
