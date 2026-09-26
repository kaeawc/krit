// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Uses of android.util.FloatMath that call none of its members, and the
// kotlin.math replacements. Neither Go nor FIR reports them: the import, a
// class literal, a type reference, a comment, and a string.
package test

import android.util.FloatMath
import kotlin.math.sqrt

fun replacement(x: Float, y: Float) = sqrt(x * x + y * y)

fun qualifiedReplacement(x: Float) = kotlin.math.sin(x)

fun javaMath(x: Double) = Math.sqrt(x)

fun classLiteral(): Class<FloatMath> = FloatMath::class.java

fun typeReference(math: FloatMath?): Boolean = math == null

// FloatMath.sqrt(x) is deprecated.
fun text() = "FloatMath.sqrt(x)"
