package com.example

import kotlin.math.sqrt

class GeometryHelper {
    fun computeDistance(x: Float, y: Float): Float {
        return kotlin.math.sqrt(x * x + y * y)
    }
}
