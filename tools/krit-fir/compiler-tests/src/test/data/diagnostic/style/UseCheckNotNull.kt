// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 12, 16, 20, 26, 30, 34, 38, 44, 49, 56, 62, 69, 75, 78
// Positives: kotlin.check called with a single `!=` null comparison of a
// nullable value, with or without a trailing lazy-message lambda.
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
    <!UseCheckNotNull!>kotlin.check(x != null)<!>
}

fun multiLineQualifier(x: Any?) {
    <!UseCheckNotNull!>kotlin<!>
        .check(x != null)
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
