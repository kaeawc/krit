// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 15x2, 17, 20, 22, 24, 27, 29, 32, 37, 41, 46, 52, 55
// Calls on the deprecated android.util.FloatMath through its simple name, the
// shape the Go rule reports: one finding per `FloatMath.` receiver, on the
// receiver's line, in every kind of container.
package test

import android.util.FloatMath

class GeometryHelper {
    fun computeDistance(x: Float, y: Float): Float {
        return <!FloatMath!>FloatMath<!>.sqrt(x * x + y * y)
    }

    fun nested(x: Float) = <!FloatMath!>FloatMath<!>.sin(<!FloatMath!>FloatMath<!>.cos(x))

    fun split(x: Float) = <!FloatMath!>FloatMath<!>
        .floor(x)

    fun chained(x: Float) = <!FloatMath!>FloatMath<!>.exp(x).toInt()

    fun template(x: Float) = "${<!FloatMath!>FloatMath<!>.pow(x, 2f)}"

    val lambda = { x: Float -> <!FloatMath!>FloatMath<!>.hypot(x, x) }

    val getter: Float
        get() = <!FloatMath!>FloatMath<!>.ceil(1.5f)

    fun default(x: Float = <!FloatMath!>FloatMath<!>.sqrt(4f)) = x

    companion object {
        val ROOT_TWO = <!FloatMath!>FloatMath<!>.sqrt(2f)
    }
}

object Holder {
    val v = <!FloatMath!>FloatMath<!>.sqrt(2f)

    fun anon() = object : Runnable {
        override fun run() {
            <!FloatMath!>FloatMath<!>.sqrt(1f)
        }
    }

    fun local(): Float {
        fun inner(x: Float) = <!FloatMath!>FloatMath<!>.sqrt(x)
        return inner(9f)
    }
}

interface Shape {
    fun area(r: Float): Float = <!FloatMath!>FloatMath<!>.pow(r, 2f)
}

val topLevel = <!FloatMath!>FloatMath<!>.floor(3.7f)
