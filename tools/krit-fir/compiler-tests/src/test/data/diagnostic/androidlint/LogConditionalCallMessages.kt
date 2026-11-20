// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 16, 17, 18, 19, 20, 21, 22, 23, 24
// Both engines report calls anywhere in messages, including templates and concatenation operands.
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
    <!LogConditional!>Log.d("Tag", value ?: compute())<!>
    <!LogConditional!>Log.d("Tag", if (flag) "literal" else compute())<!>
    <!LogConditional!>Log.d("Tag", "${compute().length}")<!>
    Log.d(compute(), "literal")
    Log.println(Log.INFO, compute(), "literal")
    Log.d("Tag", "compute()")
}
