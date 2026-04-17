package com.example

fun logState(state: String) {
    if (BuildConfig.DEBUG) {
        Log.d("TAG", "state=$state")
    }
}

fun updateState(state: String) {
    if (!BuildConfig.DEBUG) {
        persist(state)
    }
}

fun persist(state: String) {
    println(state)
}
