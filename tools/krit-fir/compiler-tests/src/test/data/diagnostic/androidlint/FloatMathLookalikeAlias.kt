// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12
// Neither side reports: java.lang.Math imported under the alias FloatMath calls
// java.lang.Math, the replacement the message recommends. Go requires an import
// of android.util.FloatMath (or a qualified receiver), so it skips the alias.
package test

import java.lang.Math as FloatMath

fun root(x: Double) = FloatMath.sqrt(x)

fun sine(x: Double) = FloatMath.sin(x)
