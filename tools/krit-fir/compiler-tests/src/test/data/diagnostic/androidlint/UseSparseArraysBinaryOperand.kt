// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (recall): tree-sitter misparses a generic constructor call on the
// right-hand side of an Elvis or `+` operator, so Go never sees a
// call_expression named HashMap there and reports nothing. Each call
// constructs a java.util.HashMap<Int, String>, so FIR reports it.
package test

fun elvis(x: Map<Int, String>?): Map<Int, String> = x ?: <!UseSparseArrays!>HashMap<Int, String>()<!>

fun plus(a: Map<Int, String>): Map<Int, String> = a + <!UseSparseArrays!>HashMap<Int, String>()<!>

fun safeCastElvis(a: Any): Map<Int, String> = a as? HashMap<Int, String> ?: <!UseSparseArrays!>HashMap<Int, String>()<!>

fun multilineElvis(x: Map<Int, String>?): Map<Int, String> {
    val y = x
        ?: <!UseSparseArrays!>HashMap<Int, String>()<!>
    return y
}

// Go parses these contexts, and both report them.
fun ifElse(c: Boolean): Map<Int, String> = if (c) <!UseSparseArrays!>HashMap<Int, String>()<!> else emptyMap()

fun equality(a: Map<Int, String>): Boolean = a == <!UseSparseArrays!>HashMap<Int, String>()<!>
