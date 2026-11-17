// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 11, 15, 20, 27, 31, 33, 37, 41, 45, 49, 53, 57, 63, 69, 75
// Positives: a throw of a new IllegalStateException with no argument or one String message.
package test.usecheckorerror

fun negated(valid: Boolean) {
    if (!valid) <!UseCheckOrError!>throw IllegalStateException("bad")<!>
}

fun noArguments(value: Int) {
    if (!(value > 0)) <!UseCheckOrError!>throw IllegalStateException()<!>
}

fun nonNegated(cond: Boolean) {
    if (cond) <!UseCheckOrError!>throw IllegalStateException()<!>
}

fun braced(cond: Boolean) {
    if (cond) {
        <!UseCheckOrError!>throw IllegalStateException("braced")<!>
    }
}

fun withOtherStatements(cond: Boolean) {
    if (cond) {
        println("log")
        <!UseCheckOrError!>throw IllegalStateException("log first")<!>
    }
}

fun elvis(x: String?): String = x ?: <!UseCheckOrError!>throw IllegalStateException("missing")<!>

fun expressionBody(): Nothing = <!UseCheckOrError!>throw IllegalStateException("never")<!>

fun inWhenBranch(x: Int): String = when (x) {
    0 -> "zero"
    else -> <!UseCheckOrError!>throw IllegalStateException("unexpected $x")<!>
}

fun stringTemplate(name: String) {
    if (name.isEmpty()) <!UseCheckOrError!>throw IllegalStateException("empty: $name")<!>
}

fun stringVariable(message: String) {
    if (message.isEmpty()) <!UseCheckOrError!>throw IllegalStateException(message)<!>
}

fun javaLangName(cond: Boolean) {
    if (cond) <!UseCheckOrError!>throw java.lang.IllegalStateException("qualified")<!>
}

fun kotlinName(cond: Boolean) {
    if (cond) <!UseCheckOrError!>throw kotlin.IllegalStateException("qualified")<!>
}

fun chained(cond: Boolean) {
    if (cond) <!UseCheckOrError!>throw IllegalStateException("chained").also { println(it) }<!>
}

fun lambdaWithOtherStatements(items: List<Int>) {
    items.forEach {
        println(it)
        <!UseCheckOrError!>throw IllegalStateException("after statement")<!>
    }
}

fun insideLoop(items: List<Int>) {
    for (item in items) {
        if (item < 0) <!UseCheckOrError!>throw IllegalStateException("negative")<!>
    }
}

class Holder(val value: String?) {
    fun member() {
        if (value == null) <!UseCheckOrError!>throw IllegalStateException("no value")<!>
    }
}
