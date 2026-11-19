// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 14, 18, 22, 28, 32, 36, 41, 46, 51, 58, 64, 71, 77, 80, 85, 89
// Positives: kotlin.check called with a `!=` null comparison of a nullable
// value as its first argument, with or without a lazy message (inside the
// parentheses or as a trailing lambda). A qualified call is reported at the
// callee name, as Go and detekt do.
package test

fun plain(x: Any?) {
    <!UseCheckNotNull!>check(x != null)<!>
}

fun reversed(x: String?) {
    <!UseCheckNotNull!>check(null != x)<!>
}

fun withMessage(x: Any?) {
    <!UseCheckNotNull!>check(x != null) { "x must not be null" }<!>
}

fun multiLineMessage(x: Any?) {
    <!UseCheckNotNull!>check(x != null)<!> {
        "x must not be null"
    }
}

fun parenthesized(x: Any?) {
    <!UseCheckNotNull!>check((x != null))<!>
}

fun namedArgument(x: Any?) {
    <!UseCheckNotNull!>check(value = x != null)<!>
}

fun qualified(x: Any?) {
    kotlin.<!UseCheckNotNull!>check(x != null)<!>
}

fun multiLineQualifier(x: Any?) {
    kotlin
        .<!UseCheckNotNull!>check(x != null)<!>
}

class Holder(val value: String?) {
    fun member() {
        <!UseCheckNotNull!>check(value != null)<!>
    }

    companion object {
        fun fromCompanion(x: Int?) {
            <!UseCheckNotNull!>check(x != null)<!>
        }
    }
}

fun insideLambda(x: Any?) {
    run {
        <!UseCheckNotNull!>check(x != null)<!>
    }
}

fun localFunction(x: Any?) {
    fun inner() {
        <!UseCheckNotNull!>check(x != null)<!>
    }
    inner()
}

val anonymous = object {
    fun validate(x: Any?) {
        <!UseCheckNotNull!>check(x != null)<!>
    }
}

val topLevelInitializer: Unit = run {
    val x: String? = System.getenv("KRIT")
    <!UseCheckNotNull!>check(x != null)<!>
}

fun asExpression(x: Any?): Unit = <!UseCheckNotNull!>check(x != null)<!>

// The lazy message may be the second argument inside the parentheses, as
// detekt reads only the first: checkNotNull(x, { .. }) is the replacement.
fun messageInParentheses(x: Any?) {
    <!UseCheckNotNull!>check(x != null, { "x must not be null" })<!>
}

fun namedMessage(x: Any?) {
    <!UseCheckNotNull!>check(value = x != null, lazyMessage = { "x must not be null" })<!>
}
