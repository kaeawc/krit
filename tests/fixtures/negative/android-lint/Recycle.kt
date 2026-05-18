package com.example

import android.content.Context
import android.content.res.TypedArray

class StyleHelper(private val context: Context) {
    fun getColor(attrs: IntArray): Int {
        val ta: TypedArray = context.obtainStyledAttributes(attrs)
        val color = ta.getColor(0, 0)
        ta.recycle()
        return color
    }

    // Regression: a short-named TypedArray `ta` is properly recycled; the
    // receiver-bound identifier-boundary check must recognise `ta.recycle()`
    // (and must not be confused into thinking some longer-named sibling
    // closed it).
    fun getColorWithSiblingResource(attrs: IntArray, other: IntArray): Int {
        val ta: TypedArray = context.obtainStyledAttributes(attrs)
        val vta: TypedArray = context.obtainStyledAttributes(other)
        val color = ta.getColor(0, 0)
        try {
            // do something with vta
            vta.getColor(0, 0)
        } finally {
            ta.recycle()
            vta.recycle()
        }
        return color
    }
}
