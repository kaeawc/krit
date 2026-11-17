// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 11, 15, 20, 27, 34, 38, 41, 45, 49, 53, 57, 61, 65, 71, 77, 83, 87
// Positives: a throw of a new IllegalArgumentException that is the whole body of an if branch or a loop.
package test.userequire

fun negated(valid: Boolean) {
    if (!valid) <!UseRequire!>throw IllegalArgumentException("bad")<!>
}

fun noArguments(value: Int) {
    if (!(value > 0)) <!UseRequire!>throw IllegalArgumentException()<!>
}

fun nonNegated(cond: Boolean) {
    if (cond) <!UseRequire!>throw IllegalArgumentException()<!>
}

fun braced(cond: Boolean) {
    if (cond) {
        <!UseRequire!>throw IllegalArgumentException("braced")<!>
    }
}

fun elseBranch(cond: Boolean): Int {
    if (cond) {
        return 1
    } else <!UseRequire!>throw IllegalArgumentException("else")<!>
}

fun elseBraced(cond: Boolean): Int {
    if (cond) {
        return 1
    } else {
        <!UseRequire!>throw IllegalArgumentException("else braced")<!>
    }
}

fun ifExpression(cond: Boolean): Int = if (cond) 1 else <!UseRequire!>throw IllegalArgumentException("expression")<!>

fun whileBody(cond: Boolean) {
    while (cond) <!UseRequire!>throw IllegalArgumentException("loop")<!>
}

fun forBody(items: List<Int>) {
    for (item in items) <!UseRequire!>throw IllegalArgumentException("item $item")<!>
}

fun doWhileBody(cond: Boolean) {
    do <!UseRequire!>throw IllegalArgumentException("do")<!> while (cond)
}

fun stringVariable(message: String) {
    if (message.isEmpty()) <!UseRequire!>throw IllegalArgumentException(message)<!>
}

fun javaLangName(cond: Boolean) {
    if (cond) <!UseRequire!>throw java.lang.IllegalArgumentException("qualified")<!>
}

fun kotlinName(cond: Boolean) {
    if (cond) <!UseRequire!>throw kotlin.IllegalArgumentException("qualified")<!>
}

fun chained(cond: Boolean) {
    if (cond) <!UseRequire!>throw IllegalArgumentException("chained").also { println(it) }<!>
}

fun commentInBraces(cond: Boolean) {
    if (cond) {
        // explain
        <!UseRequire!>throw IllegalArgumentException("commented")<!>
    }
}

class Holder(val value: String?) {
    fun member() {
        if (value == null) <!UseRequire!>throw IllegalArgumentException("no value")<!>
    }
}

// A `;` is not a statement, so the throw is still the whole braced body.
fun trailingSemicolon(cond: Boolean) {
    if (cond) { <!UseRequire!>throw IllegalArgumentException("trailing")<!>; }
}

fun leadingSemicolon(cond: Boolean) {
    if (cond) { ; <!UseRequire!>throw IllegalArgumentException("leading")<!> }
}
