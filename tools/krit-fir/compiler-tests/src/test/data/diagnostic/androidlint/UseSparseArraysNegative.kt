// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Android Lint 32.4.1 also leaves alone: other value types, an
// argument inferred from the declared type, a HashMap with an Int key, and a
// constructor reference.
package test

import android.util.SparseArray

class Cache {
    val strings = SparseArray<String>()
    val longs = SparseArray<Long>()
    val floats = SparseArray<Float>()
    val lists = SparseArray<List<Int>>()
    val inferred: SparseArray<Int> = SparseArray()
    val hashMap = HashMap<Int, String>()
    val boolMap = HashMap<Int, Boolean>()
    val factory = hashMapOf<Int, Boolean>()
    val reference: () -> SparseArray<Int> = ::SparseArray
}

class Subclass : SparseArray<Int>()

fun anonymous() = object : SparseArray<Boolean>() {}
