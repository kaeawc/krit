// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 13
// Divergence (precision): java.lang.Math imported under the alias FloatMath.
// Go matches the receiver's text and reports both calls, but each one calls
// java.lang.Math, the replacement the message recommends, so FIR does not
// report them.
package test

import java.lang.Math as FloatMath

fun root(x: Double) = FloatMath.sqrt(x)

fun sine(x: Double) = FloatMath.sin(x)
