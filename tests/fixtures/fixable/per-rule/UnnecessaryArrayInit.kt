package com.example

class Buffers {
    val ints = IntArray(16) { 0 }
    val doubles = DoubleArray(4) { 0.0 }
    val bytes = ByteArray(4, { 0 })
    val ones = IntArray(4) { 1 }
}
