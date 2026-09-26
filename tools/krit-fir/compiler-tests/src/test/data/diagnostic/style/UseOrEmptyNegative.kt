// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives the Go rule also leaves alone.
package test

fun orEmpty(x: List<String>?): List<String> = x.orEmpty()

fun nonEmpty(x: List<String>?): List<String> = x ?: listOf("default")

fun nonEmptyVararg(x: Array<String>?, rest: Array<String>): Array<out String> = x ?: arrayOf(*rest)

fun mutable(x: MutableList<String>?): MutableList<String> = x ?: mutableListOf()

fun primitiveArray(x: IntArray?): IntArray = x ?: intArrayOf()

fun defaultString(x: String?): String = x ?: "default"

fun blank(x: String?): String = x ?: " "

fun elvisReturn(x: String?): String = x ?: return "fallback"

fun elvisThrow(x: String?): String = x ?: throw IllegalStateException()

// Go skips an `emptyArray()` fallback, and never sees `emptyArray<Int>()`
// (tree-sitter parses it as `(x ?: emptyArray)<Int>()`).
fun emptyArrayFallback(x: Array<Int>?): Array<Int> = x ?: emptyArray()

fun emptyArrayTypeArguments(x: Array<Int>?): Array<Int> = x ?: emptyArray<Int>()

// A fallback of another kind than the left side has no `.orEmpty()`
// replacement. (Go does not see these either: they need explicit type
// arguments, which tree-sitter parses as a call on the Elvis.)
fun otherKind(values: List<String>?): Any = values ?: emptyMap<String, String>()

fun stringWithList(value: String?): Any = value ?: emptyList<String>()

fun <T> unbounded(value: T?): Any = value ?: emptyList<String>()

// A parenthesized fallback is not a call_expression in Go.
fun parenthesized(x: List<String>?): List<String> = x ?: (emptyList())

// A left side that reads through a safe call.
class Box(val items: List<Int>?, val name: String?, val inner: Box?)

fun safeCall(box: Box?): List<Int> = box?.items ?: emptyList()

fun safeCallChain(box: Box?): String = box?.inner?.name ?: ""

fun safeCallReceiver(box: Box?): String = box?.name?.trim() ?: ""

fun safeCallLet(x: String?): String = x?.let { it.trim() } ?: ""

fun safeCallCast(box: Box?): List<Int> = (box?.items as List<Int>?) ?: emptyList()

fun safeCallGet(items: Map<String, List<Int>>?): List<Int> = items?.get("key") ?: emptyList()

fun safeCallNestedElvis(box: Box?, fallback: String?): String = (fallback ?: box?.name) ?: ""

fun safeCallBranch(box: Box?, flag: Boolean): String = (if (flag) box?.name else null) ?: ""

fun safeCallNotNullAssertion(box: Box?): String = box?.name!!.trim().takeIf { it.isNotEmpty() } ?: ""

// An Elvis inside a string template.
fun template(x: String?): String = "value ${x ?: ""}"

fun rawTemplate(x: String?): String = """value ${x ?: ""}"""

fun templateLambda(values: List<String?>): String = "${values.map { it ?: "" }}"

fun throwableTemplate(value: Throwable?): String = "error ${value ?: ""}"
