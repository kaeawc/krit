// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: Boolean properties, names without the is prefix, and variables
// that are not property declarations. Go reports none of these either.
package test

class Flags(val enabled: Boolean) {
    val isEnabled: Boolean = true

    var isVisible: Boolean? = null

    val isInferredTrue = true

    val isInferredFalse = false

    val isComputed = enabled && isEnabled

    val isGetter: Boolean
        get() = enabled

    val isInferredGetter
        get() = !enabled

    val isLazy by lazy { enabled }

    val isLazyTyped: Boolean by lazy { enabled }

    // Only the lowercase prefix counts.
    val IsCapital: String = "cap"

    val name: String = "name"

    val visible: Int = 0

    // A function named like a property is not a property.
    fun isReady(): String = "ready"
}

class IsWrapper(val value: String)

fun notProperties(items: List<String>, pairs: List<Pair<String, Int>>, isParam: String): String {
    // Loop variables are not property declarations in Go's tree.
    for (isItem in items) {
        println(isItem)
    }
    // Neither are destructuring entries...
    val (isFirst, isSecond) = pairs.first()
    for ((isKey, isValue) in pairs) {
        println(isKey + isValue)
    }
    // ...lambda parameters...
    items.forEach { isElement -> println(isElement) }
    pairs.forEach { (isLeft, isRight) -> println(isLeft + isRight) }
    // ...catch parameters...
    try {
        println(isParam)
    } catch (isError: IllegalStateException) {
        println(isError)
    }
    // ...or a `when` subject variable.
    return when (val isSubject = items.size) {
        0 -> isFirst
        else -> isSubject.toString() + isSecond
    }
}
