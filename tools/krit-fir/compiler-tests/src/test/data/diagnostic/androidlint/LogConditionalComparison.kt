// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 14, 15, 16, 17
// An unguarded level call used as an operand of a comparison or an arithmetic
// operator is still an unconditional android.util.Log call. Go reports it
// for every operator below, and FIR matches.
package test.logconditional.comparison

import android.util.Log

fun comparisons(verbose: Boolean) {
    if (<!LogConditional!>Log.d("Tag", "greater")<!> > 0) {
        println("logged")
    }
    val less = <!LogConditional!>Log.w("Tag", "less")<!> < 0
    val atLeast = <!LogConditional!>Log.i("Tag", "at least")<!> >= 0
    val equal = <!LogConditional!>Log.e("Tag", "equal")<!> == 0
    val sum = <!LogConditional!>Log.v("Tag", "sum")<!> + 1
    if (Log.isLoggable("Tag", Log.DEBUG)) {
        val guarded = Log.d("Tag", "guarded comparison") > 0
    }
}
