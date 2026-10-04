// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 37, 44, 50, 56, 62, 69
// The operand's type after K2's data flow decides. Go's source inference
// agrees on plain smart casts (UseRequireNotNullOperandType.smartCastLocal)
// but not on the shapes below.
package test

// Divergence (Go misses): Go smart-casts the member `var` after the early
// return and skips it, but a mutable property gets no stable smart cast (as
// in UseRequireNotNullOperandType.unstableSmartCast): name is still String?
// here, and requireNotNull(name) returns it as String.
class VarHolder {
    var name: String? = null

    fun f() {
        if (name == null) return
        <!UseRequireNotNull!>require(name != null)<!>
    }
}

// Divergence: Go reports this because it does not smart-cast a `when`
// subject. The `null ->` branch returns, so wx is a non-null String in the
// else branch.
fun whenSubject(wx: String?) {
    when (wx) {
        null -> return
        else -> require(wx != null)
    }
}

// Divergence: Go reports this because it keeps the declared String? type
// after the assignment. s holds "a", a non-null String, at the require.
fun afterAssignment() {
    var s: String? = null
    s = "a"
    println(s)
    require(s != null)
}

// Divergence: Go reports this because it does not infer the elvis
// expression's type. `ex ?: return` is a non-null String.
fun elvisLocal(ex: String?) {
    val ey = ex ?: return
    require(ey != null)
}

// Divergence: Go reports this because it resolves the text "(px)" and finds
// nothing. The parenthesized operand is the non-null String px.
fun parenthesizedOperand(px: String) {
    require((px) != null)
}

// Divergence: Go reports this because it does not resolve a loop variable.
// t iterates a List<String>, so it is a non-null String.
fun loopVariable(items: List<String>) {
    for (t in items) require(t != null)
}

// Divergence: Go reports this because it does not resolve a comparison.
// `a == b` is a non-null Boolean.
fun booleanOperand(a: Int, b: Int) {
    require(a == b != null)
}

// Divergence: Go reports this because it does not resolve a Java field.
// Integer.MAX_VALUE is a primitive int, a non-null Int in Kotlin.
fun javaPrimitiveField() {
    val v = Integer.MAX_VALUE
    require(v != null)
}
