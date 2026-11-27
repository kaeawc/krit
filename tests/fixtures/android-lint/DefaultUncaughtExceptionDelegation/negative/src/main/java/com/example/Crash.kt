package com.example

fun install() {
    val previous = Thread.getDefaultUncaughtExceptionHandler()
    Thread.setDefaultUncaughtExceptionHandler { t, e -> previous?.uncaughtException(t, e) }
}
