// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: Boolean properties whose written type is not plain `Boolean`.
// Go reads the written type, so it handles these forms too: a qualified
// name, java.lang.Boolean, parentheses, and a same-file type alias. Without
// a written type Go reports nothing, even when the text contains ": ".
package test

typealias Flag = Boolean

typealias NestedFlag = Flag

class Divergence(private val items: List<String>) {
    val isQualified: kotlin.Boolean = true

    val isAliased: Flag = false

    val isAliasedNullable: Flag? = null

    val isNestedAlias: NestedFlag = true

    @Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
    val isBoxed: java.lang.Boolean? = null

    val isParen: (Boolean) = true

    val isParenNullable: (Boolean)? = null

    // The lambda's `it: String` is not the property's type.
    val isAnyEmpty = items.any { it: String -> it.isEmpty() }

    val isLabelled = "key: value".isNotEmpty()
}
