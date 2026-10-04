// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 11x2
// FloatMath resolved through a star import: Go reports the receiver by its
// text, FIR by resolution, so both report each call.
package test

import android.util.*

fun root(x: Float) = <!FloatMath!>FloatMath<!>.sqrt(x)

fun both(x: Float) = <!FloatMath!>FloatMath<!>.sin(x) + <!FloatMath!>FloatMath<!>.cos(x)
