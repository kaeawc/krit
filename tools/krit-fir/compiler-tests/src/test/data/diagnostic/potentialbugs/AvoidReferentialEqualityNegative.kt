// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class TreeNode
fun unchanged(unwrapped: TreeNode, current: TreeNode) = unwrapped !== current
fun structural(a: String, b: String) = a == b
fun nullCheck(text: String?) = text !== null
// Both engines recognize parenthesized null operands.
fun parenthesizedNull(text: String?) = text === (null)
fun reverseNull(text: String?) = null === text
fun equalsFastPath(a: String, b: String) = a === b || a.equals(b)
fun contentFastPath(a: String, b: String) = a !== b && !a.contentEquals(b)
fun normalize(text: String): String = text.trim()
fun unchangedString(text: String) = normalize(text) === text

class Holder(val text: String)
fun unchangedField(holder: Holder) = normalize(holder.text) === holder.text

class Value {
    override fun equals(other: Any?): Boolean = this === other
    override fun hashCode(): Int = 0
}

fun compare(a: String, b: String): Int {
    if (a === b) return 0
    return a.compareTo(b)
}

// Go's source resolver and FIR both bind the shadowing Any inside run {}.
// The resolverless Go fallback is conservative and does not report this.
fun shadowed(candidate: Any): Boolean {
    val text: String = "a"
    return run {
        val text: Any = candidate
        text === candidate
    }
}

// A vararg parameter is Array<out String>, not String, in both engines.
fun varargArray(vararg texts: String, candidate: Any) = texts === candidate
fun capturedVararg(vararg texts: String, candidate: Any) = run { texts === candidate }
