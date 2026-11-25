package com.example

class Repository(private val name: String) {
    private fun load(): String = name

    // Same-class lambdas, internal members and private properties do not need an accessor (Lint skips Kotlin
    // properties).
    fun lambda() = listOf(1).map { load() }

    internal fun shared() = name

    inner class Loader {
        private fun load() = "inner"
        fun run() = load() + shared() + name
    }

    companion object {
        fun describe(r: Repository) = r.load()
    }
}
