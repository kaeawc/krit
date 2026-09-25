// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: properties named is* whose type is not Boolean. Each is reported
// on the property's first line (modifier list, else val/var), the line the Go
// rule reports. Every case here is also a Go finding.
package test

class Example {
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isName: String = "hello"

    <!NonBooleanPropertyPrefixedWithIs!>var<!> isCount: Int = 0

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isNullableName: String? = null

    /**
     * KDoc is not part of the reported line.
     */
    <!NonBooleanPropertyPrefixedWithIs!>@Volatile<!>
    var isAnnotated: Long = 0L

    <!NonBooleanPropertyPrefixedWithIs!>private<!> val isPrivate: List<String> = emptyList()

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isGetter: Int
        get() = 1

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isLazy: String by lazy { "lazy" }

    // A function type is not Boolean, even when it returns one.
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isPredicate: () -> Boolean = { true }

    <!NonBooleanPropertyPrefixedWithIs!>val<!> isFlags: Map<String, Boolean> = emptyMap()

    // Go matches the prefix literally: 'issues' starts with "is".
    <!NonBooleanPropertyPrefixedWithIs!>val<!> issues: List<String> = emptyList()

    // The initializer text contains ": ", so Go reports it without a declared
    // type; the inferred type is String.
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isLabel = "key: value"

    companion object {
        <!NonBooleanPropertyPrefixedWithIs!>const<!> val isConstant: String = "c"
    }

    fun local(): String {
        <!NonBooleanPropertyPrefixedWithIs!>val<!> isLocal: String = "a"
        <!NonBooleanPropertyPrefixedWithIs!>var<!> isCounter: Int = 0
        isCounter++
        val lambda = {
            <!NonBooleanPropertyPrefixedWithIs!>val<!> isInLambda: String = "b"
            isInLambda
        }
        return isLocal + isCounter + lambda()
    }
}

interface Api {
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isAbstract: String
}

class Impl : Api {
    <!NonBooleanPropertyPrefixedWithIs!>override<!> val isAbstract: String = "impl"
}

object Holder {
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isObject: Int = 1
}

enum class Mode {
    A {
        <!NonBooleanPropertyPrefixedWithIs!>val<!> isEntry: Int = 1
    },
    B,
}

<!NonBooleanPropertyPrefixedWithIs!>val<!> isTopLevel: String = "top"

<!NonBooleanPropertyPrefixedWithIs!>val<!> String.isExtension: Int
    get() = length

<!NonBooleanPropertyPrefixedWithIs!>val<!> <T> List<T>.isFirst: T
    get() = first()

// A type parameter is not Boolean.
class Box<T>(value: T) {
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isValue: T = value
}

// Nothing declared explicitly is reported like any other declared type.
abstract class Declared {
    <!NonBooleanPropertyPrefixedWithIs!>abstract<!> val isNothing: Nothing
}

val anonymous = object {
    <!NonBooleanPropertyPrefixedWithIs!>val<!> isInAnonymous: String = "anon"
}

fun localClass(): Int {
    class Local {
        <!NonBooleanPropertyPrefixedWithIs!>val<!> isInLocalClass: Int = 1
    }
    return Local().isInLocalClass
}
