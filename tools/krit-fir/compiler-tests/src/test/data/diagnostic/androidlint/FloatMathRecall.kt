// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 29
// Divergence (recall): uses of android.util.FloatMath that Go mostly misses,
// each a true positive FIR resolves. Go reports a navigation expression whose
// receiver is `FloatMath` or the qualified `android.util.FloatMath` (go-lines
// 18 and 30), so it misses:
// - an import alias (`FM.sqrt(x)`) or a typealias (`Fm.sqrt(x)`);
// - a statically imported member (`sqrt(x)`), which has no receiver at all;
// - a callable reference (`FloatMath::sqrt`), which is not a navigation
//   expression.
// Each still calls (or references) a FloatMath member, so the message is true.
package test

import android.util.FloatMath as FM
import android.util.FloatMath.sqrt

fun qualified(x: Float) = <!FloatMath!>android.util.FloatMath<!>.sin(x)

fun aliased(x: Float) = <!FloatMath!>FM<!>.floor(x)

typealias Fm = android.util.FloatMath

fun typeAliased(x: Float) = <!FloatMath!>Fm<!>.hypot(x, x)

fun staticImport(x: Float) = <!FloatMath!>sqrt<!>(x)

fun reference(): (Float) -> Float = <!FloatMath!>FM<!>::ceil

fun qualifiedReference(): (Float) -> Float = <!FloatMath!>android.util.FloatMath<!>::exp

fun mapped(values: List<Float>) = values.map(<!FloatMath!>FM<!>::sqrt)
