package com.example

object Handlers {
    fun setDefaultUncaughtExceptionHandler(h: Any?) {}
}

fun lookalike() = Handlers.setDefaultUncaughtExceptionHandler(null)
