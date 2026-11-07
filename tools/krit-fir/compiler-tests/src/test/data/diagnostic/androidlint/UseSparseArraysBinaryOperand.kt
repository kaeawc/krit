// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 28
// Divergence (recall): tree-sitter misparses a generic constructor call on the
// right-hand side of an Elvis operator, so Go never sees a
// call_expression named HashMap there and reports nothing. Each call
// constructs a java.util.HashMap<Int, String>, so FIR reports it.
package test

fun elvis(x: Map<Int, String>?) {
    x ?: <!UseSparseArrays!>HashMap<Int, String>()<!>
}

fun safeCastElvis(a: Any) {
    val result = a as? HashMap<Int, String> ?: <!UseSparseArrays!>HashMap<Int, String>()<!>
    result.size
}

fun multilineElvis(x: Map<Int, String>?) {
    x
        ?: <!UseSparseArrays!>HashMap<Int, String>()<!>
}

// Go parses these contexts, and both report them.
fun ifElse(c: Boolean) {
    if (c) <!UseSparseArrays!>HashMap<Int, String>()<!> else emptyMap()
}

fun equality(a: Map<Int, String>): Boolean = a == <!UseSparseArrays!>HashMap<Int, String>()<!>
