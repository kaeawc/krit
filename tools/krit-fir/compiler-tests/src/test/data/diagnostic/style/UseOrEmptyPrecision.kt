// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 11, 13, 16, 18, 20, 28, 33, 37
// Go reports every Elvis below; `.orEmpty()` does not replace any of them.
package test

// The left side has no `.orEmpty()` for this fallback: an `Any?` or
// `CharSequence?` with `""` (only `String?.orEmpty()` exists), or an
// Iterable.
fun any(value: Any?): Any = value ?: ""

fun anyLookup(json: Map<String, Any?>): Any = json["key"] ?: ""

fun charSequence(value: CharSequence?): CharSequence = value ?: ""

// `x.orEmpty()` on a list returns an empty list, not "".
fun listWithString(x: List<String>?): Any = x ?: ""

fun iterable(values: Iterable<String>?): Iterable<String> = values ?: emptyList()

fun <T : Iterable<String>> iterableBound(values: T?): Iterable<String> = values ?: emptyList()


// The fallback does not resolve to the stdlib empty value.
object Defaults {
    fun emptyList(): List<String> = listOf("default")
}

fun memberLookalike(x: List<String>?): List<String> = x ?: Defaults.emptyList()

class Local {
    private fun listOf(): List<String> = kotlin.collections.listOf("default")

    fun get(x: List<String>?): List<String> = x ?: listOf()
}

// A trailing lambda is an element, so the list is not empty.
fun lambdaElement(x: List<() -> Unit>?): List<() -> Unit> = x ?: listOf { }
