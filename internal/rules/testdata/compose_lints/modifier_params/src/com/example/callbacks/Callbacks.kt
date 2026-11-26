package com.example.callbacks

typealias OnSelect = (String) -> Unit

typealias MaybeSelect = OnSelect?

fun interface Listener {
    fun onEvent()
}
