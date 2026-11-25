package com.example

class Repository private constructor(private val name: String) {
    private fun load(): String = name

    inner class Loader {
        fun run() = load()
    }

    class Factory {
        fun create() = Repository("default")
    }

    fun fromAnonymous() = object : Runnable {
        override fun run() {
            load()
        }
    }
}
