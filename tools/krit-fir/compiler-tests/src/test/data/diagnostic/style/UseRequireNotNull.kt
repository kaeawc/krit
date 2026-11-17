// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 15, 20, 24, 28, 32, 38, 44, 48, 53, 58, 63, 68, 72, 80, 86, 90, 93, 98, 105, 109
// Positive: kotlin.require(x != null) on a nullable operand should trigger
// UseRequireNotNull.
package test

class Holder(val value: String?, val name: String)

fun nullableParam(x: Any?) {
    <!UseRequireNotNull!>require(x != null)<!>
    println(x)
}

fun reversed(x: String?) {
    <!UseRequireNotNull!>require(null != x)<!>
}

// A trailing message lambda is not a value argument.
fun withMessage(x: Any?) {
    <!UseRequireNotNull!>require(x != null) { "x must not be null" }<!>
}

fun named(x: Any?) {
    <!UseRequireNotNull!>require(value = x != null)<!>
}

fun parenthesized(x: Any?) {
    <!UseRequireNotNull!>require((x != null))<!>
}

fun qualified(x: Any?) {
    kotlin.<!UseRequireNotNull!>require(x != null)<!>
}

// Reported at the callee name after the qualifier, as Go and detekt do.
fun multiline(x: Any?) {
    kotlin
        .<!UseRequireNotNull!>require(<!>
            x != null,
        )
}

fun nullableProperty(h: Holder) {
    <!UseRequireNotNull!>require(h.value != null)<!>
}

fun safeCall(h: Holder?) {
    <!UseRequireNotNull!>require(h?.name != null)<!>
}

fun localVal(env: Map<String, String>) {
    val s: String? = env["A"]
    <!UseRequireNotNull!>require(s != null)<!>
}

// Java platform types can be null.
fun platformType() {
    <!UseRequireNotNull!>require(System.getenv("HOME") != null)<!>
}

fun inferredPlatform() {
    val home = System.getenv("HOME")
    <!UseRequireNotNull!>require(home != null)<!>
}

class Member(private val field: String?) {
    fun validate() {
        <!UseRequireNotNull!>require(field != null)<!>
    }

    fun viaThis() {
        <!UseRequireNotNull!>require(this.field != null)<!>
    }
}

object Cache {
    var cached: String? = null

    fun ensure() {
        <!UseRequireNotNull!>require(cached != null)<!>
    }
}

fun inLambda(x: Any?) {
    run {
        <!UseRequireNotNull!>require(x != null)<!>
    }
}

fun expressionBody(x: Any?) = <!UseRequireNotNull!>require(x != null)<!>

fun <T : Any> nullableOfBounded(x: T?) {
    <!UseRequireNotNull!>require(x != null)<!>
}

fun inAnonymousObject(x: Any?) = object {
    fun go() {
        <!UseRequireNotNull!>require(x != null)<!>
    }
}

// The lazy message may be the second argument inside the parentheses, as
// detekt reads only the first: requireNotNull(x, { .. }) is the replacement.
fun lambdaInParentheses(x: Any?) {
    <!UseRequireNotNull!>require(x != null, { "x must not be null" })<!>
}

fun namedMessage(x: Any?) {
    <!UseRequireNotNull!>require(value = x != null, lazyMessage = { "x must not be null" })<!>
}
