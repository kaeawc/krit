package com.example

fun emitDebugLog() {
    if (BuildConfig.DEBUG) {
        println("debug only")
    }
}
