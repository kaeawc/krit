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

    // Regression: a leaked short-named TypedArray `ta` must FIRE even when a
    // longer-named sibling `vta.recycle()` is present in the same scope.
    // The substring scan that this rule used to use matched `vta.recycle()`
    // as evidence that `ta` had been recycled.
    fun getColorsWithLookalike(attrs: IntArray, other: IntArray): Int {
        val ta: TypedArray = context.obtainStyledAttributes(attrs)
        val vta: TypedArray = context.obtainStyledAttributes(other)
        val color = ta.getColor(0, 0)
        vta.recycle()
        // ta is never recycled
        return color
    }
}
