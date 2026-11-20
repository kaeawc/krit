// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 16, 17, 18, 19, 20, 21, 22
// Go reports direct calls and calls that are direct template or concatenation entries; FIR follows Lint semantics.
package test.logconditional.calls

import android.util.Log

private fun compute(): String = "computed"

class User(val name: String) {
    fun name(): String = name
}

fun callMessages(user: User, value: String?, flag: Boolean) {
    <!LogConditional!>Log.d("Tag", ((compute())))<!>
    <!LogConditional!>Log.d("Tag", "x" + compute())<!>
    <!LogConditional!>Log.d("Tag", compute() + "x")<!>
    <!LogConditional!>Log.d("Tag", "x" + (user.name + compute()))<!>
    <!LogConditional!>Log.d("Tag", "${user.name()}")<!>
    <!LogConditional!>Log.d("Tag", """${user.name()}""")<!>
    <!LogConditional!>Log.d("Tag", user.toString())<!>
    <!LogConditional!>Log.d("Tag", user.name() ?: "fallback")<!>
    <!LogConditional!>Log.d("Tag", value ?: compute())<!> // FIR reports; Go ignores a call only on Elvis's right side.
    <!LogConditional!>Log.d("Tag", if (flag) "literal" else compute())<!> // FIR reports; Go does not inspect calls nested in conditionals.
    <!LogConditional!>Log.d("Tag", "${compute().length}")<!> // FIR reports; Go treats the interpolation as a property read.
    Log.d(compute(), "literal")
    Log.println(Log.INFO, compute(), "literal")
    Log.d("Tag", "compute()")
}
