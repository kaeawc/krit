package com.example

fun logState(state: String) {
    if (!BuildConfig.DEBUG) {
        Log.d("TAG", "state=$state")
    }
}
