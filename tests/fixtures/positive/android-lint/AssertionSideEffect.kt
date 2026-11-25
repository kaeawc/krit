package com.example

import java.io.File

class Registry(private val dir: File) {
    private var count = 0

    fun check(items: java.util.ArrayList<String>) {
        assert(count++ > 0)
        assert(dir.mkdirs())
        assert(items.add("x"))
        assert(ArrayList<String>().add("y"))
        assert(bump())
        assert(if (--count > 0) true else false)
    }

    private fun bump(): Boolean {
        count += 1
        return true
    }
}
