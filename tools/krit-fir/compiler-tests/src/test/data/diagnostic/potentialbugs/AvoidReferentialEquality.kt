// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 5, 6, 7, 8, 9, 13, 18, 28, 34
package test

fun strings(a: String, b: String) = <!AvoidReferentialEquality!>a === b<!>
fun nullable(a: String?, b: String?) = <!AvoidReferentialEquality!>a !== b<!>
fun literal(candidate: Any) = <!AvoidReferentialEquality!>candidate === "a"<!>
fun template(value: Any, candidate: Any) = <!AvoidReferentialEquality!>"$value" !== candidate<!>
fun rawTemplate(value: Any, candidate: Any) = <!AvoidReferentialEquality!>"""$value""" === candidate<!>

fun explicit(candidate: Any): Boolean {
    val text: String = "a"
    return <!AvoidReferentialEquality!>text === candidate<!>
}

fun inferred(candidate: Any): Boolean {
    val text = "a"
    return <!AvoidReferentialEquality!>text !== candidate<!>
}

// Go stays silent when its resolver misses this annotation; only literals have a fallback.
fun qualified(candidate: Any): Boolean {
    val text: kotlin.String? = "a"
    return <!AvoidReferentialEquality!>text === candidate<!>
}

fun multiline(a: String, b: String): Boolean {
    return <!AvoidReferentialEquality!>a ===<!>
        b
}

// Both engines use a String smart cast inside this branch.
fun smartCast(text: Any, candidate: Any): Boolean {
    if (text is String) return <!AvoidReferentialEquality!>text === candidate<!>
    return false
}
