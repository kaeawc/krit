package com.example

const val ZERO = 0

class Buffers {
    val ints = IntArray(16) { 0 }
    val longs = LongArray(4) { 0L }
    val floats = FloatArray(4) { 0f }
    val doubles = DoubleArray(4) { 0.0 }
    val shorts = ShortArray(4) { _ -> 0 }
    val bytes = kotlin.ByteArray(4, { 0 })
    val folded = IntArray(4) { 1 - 1 }
    val named = IntArray(4) { ZERO }
    val labeled = IntArray(4) { return@IntArray 0 }
}
