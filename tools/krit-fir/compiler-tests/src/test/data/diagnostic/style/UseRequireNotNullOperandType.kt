// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 21, 25, 45, 55, 61, 65, 74, 80, 84, 88
// The compared operand's type decides, whatever the expression's shape. Go
// skips a non-nullable operand only when it can resolve it as a plain name;
// FIR reads the operand's resolved type.
package test

class Box(val name: String, val label: String?)

// Divergence: Go reports this because it cannot resolve `b.name`, a dotted
// expression. `name` is a non-null String, so the comparison is senseless and
// requireNotNull is not the fix; Go skips the same shape for a plain name
// (UseRequireNotNullNegative.nonNullParam).
fun nonNullProperty(b: Box) {
    require(b.name != null)
}

// Divergence: Go reports this for the same reason; `first()` returns a
// non-null String.
fun nonNullCall(items: List<String>) {
    require(items.first() != null)
}

fun nullableCall(items: List<String>) {
    <!UseRequireNotNull!>require(items.firstOrNull() != null)<!>
}

// Divergence (Go misses): Go resolves `x: T` as a non-nullable type and skips
// it, but T's bound is Any?, so x can be null and requireNotNull(x) returns it
// as T & Any.
fun <T> nullableTypeParameter(x: T) {
    <!UseRequireNotNull!>require(x != null)<!>
}

// Both skip: the early return smart-casts x to non-null.
fun smartCastLocal(x: Any?) {
    if (x == null) return
    require(x != null)
}

// Divergence: Go reports this because it cannot resolve `b.label`. The stable
// `val` is smart-cast to non-null by the early return, as in smartCastLocal.
fun smartCastProperty(b: Box) {
    if (b.label == null) return
    require(b.label != null)
}

object Settings {
    var current: String? = null
}

// A `var` property gets no stable smart cast: it can be null again here.
fun unstableSmartCast() {
    if (Settings.current == null) return
    <!UseRequireNotNull!>require(Settings.current != null)<!>
}

typealias MaybeName = String?

fun nullableAlias(name: MaybeName) {
    <!UseRequireNotNull!>require(name != null)<!>
}

fun nullableLambdaParameter(items: List<String?>) {
    items.forEach { <!UseRequireNotNull!>require(it != null)<!> }
}

// Both skip: `it` is a non-null String.
fun nonNullLambdaParameter(items: List<String>) {
    items.forEach { require(it != null) }
}

fun String?.nullableReceiver() {
    <!UseRequireNotNull!>require(this != null)<!>
}

// Divergence: Go reports this because it does not resolve `this`; the
// receiver is a non-null String.
fun String.nonNullReceiver() {
    require(this != null)
}

fun nullableFunction(callback: (() -> Unit)?) {
    <!UseRequireNotNull!>require(callback != null)<!>
}

fun javaGetter(file: java.io.File) {
    <!UseRequireNotNull!>require(file.parent != null)<!>
}
