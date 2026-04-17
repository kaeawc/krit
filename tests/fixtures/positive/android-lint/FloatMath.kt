package com.example

import android.util.FloatMath

class GeometryHelper {
    fun computeDistance(x: Float, y: Float): Float {
        return FloatMath.sqrt(x * x + y * y)
    }
}
