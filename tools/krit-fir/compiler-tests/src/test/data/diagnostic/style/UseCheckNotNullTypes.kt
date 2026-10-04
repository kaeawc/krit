// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 30, 34, 38, 42, 46, 53, 57, 61
// The compared value's type decides the finding: a type that can hold null
// (nullable, platform, type parameter with a nullable bound) is reported, a
// type that cannot is skipped, whatever the expression's shape.
package test

class Box(val nullable: String?, val present: String)

fun nullableCall(): String? = null

fun presentCall(): String = ""

// Positive Go misses: an unbounded T can hold null. Go resolves the declared
// type `T` as a non-null type and skips it.
fun <T> typeParameter(x: T) {
    <!UseCheckNotNull!>check(x != null)<!>
}

fun <T : Any> nonNullTypeParameter(x: T) {
    check(x != null)
}

fun platformType() {
    <!UseCheckNotNull!>check(System.getenv("KRIT") != null)<!>
}

fun platformLocal() {
    val home = System.getProperty("user.home")
    <!UseCheckNotNull!>check(home != null)<!>
}

fun nullableProperty(box: Box) {
    <!UseCheckNotNull!>check(box.nullable != null)<!>
}

fun nullableCallResult() {
    <!UseCheckNotNull!>check(nullableCall() != null)<!>
}

fun nullableSafeCall(box: Box?) {
    <!UseCheckNotNull!>check(box?.present != null)<!>
}

fun String?.nullableReceiver() {
    <!UseCheckNotNull!>check(this != null)<!>
}

// Not nullable: there is no nullable value to pass to checkNotNull, and the
// comparison is always true. Go resolves only a plain name, so it cannot see
// the type of a property chain, a call or `this` and reports these three.
fun nonNullProperty(box: Box) {
    check(box.present != null)
}

fun nonNullCallResult() {
    check(presentCall() != null)
}

fun String.nonNullReceiver() {
    check(this != null)
}

// Go infers the local's type from its initializer and skips it too.
fun nonNullInferredLocal() {
    val value = presentCall()
    check(value != null)
}
