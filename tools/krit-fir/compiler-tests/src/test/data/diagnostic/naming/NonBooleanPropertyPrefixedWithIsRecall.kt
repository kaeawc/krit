// RENDER_DIAGNOSTICS_FULL_TEXT
// True positives Go misses: each property below is named is* and is not a
// Boolean, so Go's message is true of it, but Go does not report it.
package test

class Recall(
    // Go misses these: Go visits property declarations, not primary-constructor
    // `val`/`var` parameters, which are properties too.
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isId: String,
    <!NonBooleanPropertyPrefixedWithIs!>private<!> var isCount: Int,
    // A plain constructor parameter is not a property.
    isPlain: String,
) {
    // Go misses these: no declared type and no ": " in the declaration, so Go
    // never looks at the type. The inferred types are String, Int, List<String>.
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isName = "hello"

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isCounted = isCount + 1

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isList = listOf(isPlain)

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isLazy by lazy { isId.length }

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isGetter get() = isId

    // Go misses this: the declared type has no space after the colon, and Go
    // requires the text ": ".
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isTight:String = "tight"

    fun local(): Int {
        // Go misses this local for the same reason: the inferred type is Int.
        <!NonBooleanPropertyPrefixedWithIs!>val<!> isSize = isList.size
        return isSize
    }

    // Go misses this: the backticks are part of Go's identifier text, so the
    // name does not start with "is"; the property is named isQuoted.
    <!NonBooleanPropertyPrefixedWithIs!>val<!> `isQuoted`: String = "quoted"

    class Shadow {
        class Boolean

        // Go misses this: the declared type text is `Boolean`, but it names
        // the nested class Shadow.Boolean, not kotlin.Boolean.
        <!NonBooleanPropertyPrefixedWithIs!>val<!> isShadowed: Boolean = Boolean()
    }
}
