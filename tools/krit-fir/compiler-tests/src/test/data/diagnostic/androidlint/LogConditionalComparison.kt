// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15
// The enclosing if guards its condition call; w/e never report. Other operands do work.
package test.logconditional.comparison

import android.util.Log

fun comparisons(verbose: Boolean) {
    if (Log.d("Tag", compute()) > 0) {
        println("logged")
    }
    val less = Log.w("Tag", compute()) < 0
    val atLeast = <!LogConditional!>Log.i("Tag", compute())<!> >= 0
    val equal = Log.e("Tag", compute()) == 0
    val sum = <!LogConditional!>Log.v("Tag", compute())<!> + 1
    if (Log.isLoggable("Tag", Log.DEBUG)) {
        val guarded = Log.d("Tag", compute()) > 0
    }
}

private fun compute(): String = "computed"
