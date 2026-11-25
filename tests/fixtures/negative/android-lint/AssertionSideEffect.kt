package com.example

class Registry {
    private var count = 0

    fun check(items: MutableList<String>, flags: MutableSet<Int>) {
        // kotlin.collections members and pure expressions are not side effects for Lint.
        assert(count > 0)
        assert(items.add("x"))
        assert(flags.remove(1))
        assert(mutableListOf<Int>().add(1))
        assert(!(count > 0))
        assert(isPure())
        assert(usesLocals())
        // assert(count++ > 0) in a comment
        val text = "assert(count++ > 0)"
    }

    private fun isPure(): Boolean = count > 0

    private fun usesLocals(): Boolean {
        var local = 0
        local++
        return local > 0
    }
}

object Lookalike {
    // A local assert that is not kotlin.assert.
    private fun assert(value: Boolean) = value

    fun run(counter: IntArray) {
        var c = 0
        assert(c++ > 0)
    }
}
