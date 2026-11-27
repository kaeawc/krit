package com.example

fun again() {
    Thread.setDefaultUncaughtExceptionHandler(null)
}

object Handlers {
    fun setDefaultUncaughtExceptionHandler(h: Any?) {}
}

fun lookalike() = Handlers.setDefaultUncaughtExceptionHandler(null)
