// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 17, 20, 24, 29, 31, 35, 39
// Go findings FIR drops: each property below IS a Boolean, so Go's message
// ("Non-Boolean property ...") is false of the code. Go decides Boolean-ness
// from the declared type's text (only `Boolean` / `Boolean?`) or a literal
// true/false initializer, and otherwise reports any declaration whose text
// contains ": ".
package test

typealias Flag = Boolean

class Divergence(private val items: List<String>) {
    // Go reports this: the declared type text is `kotlin.Boolean`, not `Boolean`.
    val isQualified: kotlin.Boolean = true

    // Go reports this: the declared type text is `Flag`, an alias of Boolean.
    val isAliased: Flag = false

    // Go reports this: the declared type text is `Flag?`.
    val isAliasedNullable: Flag? = null

    // Go reports this: the declared type text is `java.lang.Boolean?`, the
    // boxed Boolean.
    @Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
    val isBoxed: java.lang.Boolean? = null

    // Go reports these: the declared type text is the parenthesized `(Boolean)`
    // / `(Boolean)?`, not `Boolean` / `Boolean?`.
    val isParen: (Boolean) = true

    val isParenNullable: (Boolean)? = null

    // Go reports this: no declared type, but the lambda's `it: String`
    // contains ": "; the inferred type is Boolean.
    val isAnyEmpty = items.any { it: String -> it.isEmpty() }

    // Go reports this: no declared type, but the string contains ": "; the
    // inferred type is Boolean.
    val isLabelled = "key: value".isNotEmpty()
}
