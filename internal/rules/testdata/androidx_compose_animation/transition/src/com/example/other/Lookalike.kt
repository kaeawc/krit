package com.example.other

class Transition<S> {
    fun animateFloat(label: String, block: (S) -> Float): Float = 0f
}

fun uses(t: Transition<Int>) {
    t.animateFloat("a") { 1f }
}
