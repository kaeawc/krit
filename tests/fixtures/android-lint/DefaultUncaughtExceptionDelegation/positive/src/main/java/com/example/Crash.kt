package com.example

fun install() {
    Thread.setDefaultUncaughtExceptionHandler { _, e -> println(e) }
    java.lang.Thread.setDefaultUncaughtExceptionHandler(null)
}
