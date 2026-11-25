package com.example

class Buffers {
    // The initializer computes or differs from the default value.
    val indices = IntArray(16) { it }
    val ones = IntArray(16) { 1 }
    val flags = BooleanArray(4) { false }
    val chars = CharArray(4) { ' ' }
    val unsigned = UIntArray(4) { 0u }
    val boxed = Array(4) { 0 }
    val noLambda = IntArray(16)
    val sideEffect = IntArray(4) { println(it); 0 }
    // "IntArray(4) { 0 }" in a string is not a call.
    val text = "IntArray(4) { 0 }"
}

// A class of the file named like a kotlin array.
class LongArray(size: Int, init: (Int) -> Long)

val custom = LongArray(4) { 0L }
