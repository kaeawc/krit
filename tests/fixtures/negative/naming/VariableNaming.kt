package com.example.naming

fun example() {
    val goodName = 1
    var anotherGood = "correct"
    val x = 42
    val _ = computeValue()
}

fun install(view: View) {
    view.setOnClickListener(object : Listener {
        private val DEBUG_TAP_TARGET = 8

        override fun onClick(view: View) = Unit
    })
}

fun computeValue(): String = "value"

class View {
    fun setOnClickListener(listener: Listener) = Unit
}

interface Listener {
    fun onClick(view: View)
}
