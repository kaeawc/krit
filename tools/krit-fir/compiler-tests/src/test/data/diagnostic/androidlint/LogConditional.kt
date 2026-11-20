// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 16, 17, 36, 38, 42, 43, 44, 45, 46, 61, 63, 76, 80, 81, 83, 84, 87, 92, 93, 94, 96, 99, 101, 103, 129, 130
// FIR follows Android Lint 32.4.1; Go requires calls in messages. Nearest guard wins in both.
package test.logconditional

import android.util.Log

private const val TAG = "Tag"
private const val CONSTANT = "constant"
private fun compute(): String = "computed"
private val topValue = "value"
private val atTopLevel = <!LogConditional!>Log.i(TAG, compute())<!>

class Messages {
    val field = "field"
    val inMember = <!LogConditional!>Log.d(TAG, compute())<!>
    init { <!LogConditional!>Log.d(TAG, compute())<!> }
    fun messages(value: String, flag: Boolean, error: Throwable) {
        Log.d(TAG, "literal")
        Log.d(TAG, value)
        Log.d(TAG, field)
        Log.d(TAG, CONSTANT)
        Log.d(TAG, "a" + "b")
        Log.d(TAG, "x$CONSTANT")
        Log.d(TAG, "${1}")
        Log.d(TAG, "${2 * 3}")
        Log.d(TAG, "${1 > 0}")
        Log.d(TAG, "x" + CONSTANT)
        val local = "literal"
        Log.d(TAG, local + "suffix")
        <!LogConditional!>Log.d(TAG, "x$local")<!> // FIR reports non-const template references; Go is silent.
        <!LogConditional!>Log.d(TAG, "x$topValue")<!> // FIR reports; Go is silent without a message call.
        <!LogConditional!>Log.d(TAG, "x$field")<!> // FIR reports; Go is silent without a message call.
        <!LogConditional!>Log.d(TAG, "x$value")<!> // FIR reports; Go is silent without a message call.
        <!LogConditional!>Log.d(TAG, "x" + value)<!> // FIR reports non-const concatenation; Go is silent.
        <!LogConditional!>Log.d(TAG, (compute()))<!>
        <!LogConditional!>Log.d(TAG, this.field)<!> // FIR reports a Kotlin property read; Go is silent.
        <!LogConditional!>Log.d(TAG, value.length.toString())<!>
        <!LogConditional!>Log.d(TAG, value ?: "fallback")<!> // FIR reports; Go is silent without a call.
        <!LogConditional!>Log.d(TAG, value!!)<!> // FIR reports; Go is silent without a call.
        <!LogConditional!>Log.d(TAG, if (flag) value else "fallback")<!> // FIR reports; Go is silent without a call.
        <!LogConditional!>Log.d(TAG, compute())<!>
        <!LogConditional!>Log.d(TAG, compute(), error)<!>
        <!LogConditional!>Log.v(TAG, compute())<!>
        <!LogConditional!>Log.i(TAG, compute())<!>
        <!LogConditional!>Log.println(Log.INFO, TAG, compute())<!>
        Log.println(Log.INFO, compute(), "literal")
        Log.w(TAG, compute())
        Log.e(TAG, compute(), error)
        Log.wtf(TAG, compute())
    }
    companion object { const val NAME = "name" }
}

fun guards(flag: Boolean, number: Int, items: List<String>) {
    if (flag) Log.d(TAG, compute())
    if (number > 0) Log.d(TAG, compute())
    if (false) Log.d(TAG, compute())
    if (!flag) Log.d(TAG, compute())
    if (flag || number > 0) Log.d(TAG, compute())
    if (true) <!LogConditional!>Log.d(TAG, compute())<!>
    if (flag) {
        if (true) <!LogConditional!>Log.d(TAG, compute())<!>
    } else {
        Log.d(TAG, compute())
    }
    if (true) {
        Log.d(TAG, "literal")
    } else if (flag) {
        Log.d(TAG, compute())
    } else {
        Log.d(TAG, compute())
    }
    when (number) {
        1, 2 -> Log.d(TAG, compute())
        else -> <!LogConditional!>Log.d(TAG, compute())<!>
    }
    when {
        flag -> Log.d(TAG, compute())
        true -> <!LogConditional!>Log.d(TAG, compute())<!>
        else -> <!LogConditional!>Log.d(TAG, compute())<!>
    }
    when { (true) -> <!LogConditional!>Log.d(TAG, compute())<!> }
    when (<!LogConditional!>Log.d(TAG, compute())<!>) { else -> 0 }
    when { Log.d(TAG, compute()) > 0 -> 0; else -> 0 }
    if (flag) {
        run { <!LogConditional!>Log.d(TAG, compute())<!> }
        items.forEach { <!LogConditional!>Log.d(TAG, "x$it")<!> } // FIR reports; Go is silent without a message call.
        val callback = { Log.d(TAG, compute()) }
        val anonymous = fun() { Log.d(TAG, compute()) }
        fun local() { Log.d(TAG, compute()) }
        class Local { val value = <!LogConditional!>Log.d(TAG, compute())<!> }
        val holder = object { val value = <!LogConditional!>Log.d(TAG, compute())<!> }
        println(<!LogConditional!>Log.d(TAG, compute())<!>)
    }
    val callback = { <!LogConditional!>Log.d(TAG, compute())<!> }
    run { if (flag) Log.d(TAG, compute()) }
    for (item in items) { <!LogConditional!>Log.d(TAG, "x$item")<!> } // FIR reports; Go is silent without a message call.
    while (flag) { <!LogConditional!>Log.d(TAG, compute())<!> }
    try {
        <!LogConditional!>Log.d(TAG, compute())<!>
    } catch (error: Exception) {
        <!LogConditional!>Log.d(TAG, compute())<!>
    }
    Log.d(TAG, Messages.NAME)
    Log.d(TAG, "x${Messages.NAME}")
}

class Initializers(flag: Boolean) {
    init {
        if (flag) {
            val result = Log.d(TAG, compute())
            val callback = { Log.d(TAG, compute()) }
            fun trace() { Log.d(TAG, compute()) }
        }
    }
    constructor(number: Int) : this(true) {
        if (number > 0) {
            val result = Log.d(TAG, compute())
            val callback = { Log.d(TAG, compute()) }
            fun trace() { Log.d(TAG, compute()) }
        }
    }
}

fun receiverGuards(flag: Boolean) {
    if (flag) Log.d(TAG, compute()).also { println(it) }
    if (flag) Log.d(TAG, compute()).also { println(it) }.toString()
    <!LogConditional!>Log.d(TAG, compute())<!>.also { println(it) }
    if (flag) println(<!LogConditional!>Log.d(TAG, compute())<!>.also { println(it) })
}
