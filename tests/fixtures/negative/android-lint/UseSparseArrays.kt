package com.example

import android.util.SparseArray

class Cache {
    val map = SparseArray<String>()
    val longs = SparseArray<Long>()
    val inferred: SparseArray<Int> = SparseArray()
    val hashed = HashMap<Int, String>()
}
