package com.example

import android.content.Context
import android.content.res.TypedArray

class StyleHelper(private val context: Context) {
    fun getColor(attrs: IntArray): Int {
        val ta: TypedArray = context.obtainStyledAttributes(attrs)
        val color = ta.getColor(0, 0)
        // Missing cleanup call
        return color
    }
}
