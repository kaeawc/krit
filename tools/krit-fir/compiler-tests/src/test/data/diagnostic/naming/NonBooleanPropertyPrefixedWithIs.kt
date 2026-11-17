// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 11, 13, 20, 22, 24, 27, 30, 33, 35, 37, 40, 43, 47, 48, 51, 59, 63, 67, 72, 77, 79, 84, 87, 92, 96, 101
// Positive: properties named `is` + uppercase letter whose type is not
// Boolean. Each is reported on the property's name, the line the Go rule
// reports. Every case here is also a Go finding.
package test

class Example {
    val <!NonBooleanPropertyPrefixedWithIs!>isName<!>: String = "hello"

    var <!NonBooleanPropertyPrefixedWithIs!>isCount<!>: Int = 0

    val <!NonBooleanPropertyPrefixedWithIs!>isNullableName<!>: String? = null

    /**
     * KDoc and an annotation on their own lines do not move the finding off
     * the name's line.
     */
    @Volatile
    var <!NonBooleanPropertyPrefixedWithIs!>isAnnotated<!>: Long = 0L

    private val <!NonBooleanPropertyPrefixedWithIs!>isPrivate<!>: List<String> = emptyList()

    val <!NonBooleanPropertyPrefixedWithIs!>isGetter<!>: Int
        get() = 1

    val <!NonBooleanPropertyPrefixedWithIs!>isLazy<!>: String by lazy { "lazy" }

    // A function type counts only when it returns Boolean.
    val <!NonBooleanPropertyPrefixedWithIs!>isProducer<!>: () -> String = { "" }

    // ...and a function returning a function returning Boolean does not.
    val <!NonBooleanPropertyPrefixedWithIs!>isCurried<!>: () -> () -> Boolean = { { true } }

    val <!NonBooleanPropertyPrefixedWithIs!>isFlags<!>: Map<String, Boolean> = emptyMap()

    val <!NonBooleanPropertyPrefixedWithIs!>isNullableList<!>: List<Boolean>? = null

    // Any uppercase letter after `is` counts, as in Character.isUpperCase.
    val <!NonBooleanPropertyPrefixedWithIs!>isÜmlaut<!>: Int = 1

    companion object {
        const val <!NonBooleanPropertyPrefixedWithIs!>isConstant<!>: String = "c"
    }

    fun local(): String {
        val <!NonBooleanPropertyPrefixedWithIs!>isLocal<!>: String = "a"
        var <!NonBooleanPropertyPrefixedWithIs!>isCounter<!>: Int = 0
        isCounter++
        val lambda = {
            val <!NonBooleanPropertyPrefixedWithIs!>isInLambda<!>: String = "b"
            isInLambda
        }
        return isLocal + isCounter + lambda()
    }
}

interface Api {
    val <!NonBooleanPropertyPrefixedWithIs!>isAbstract<!>: String
}

class Impl : Api {
    override val <!NonBooleanPropertyPrefixedWithIs!>isAbstract<!>: String = "impl"
}

object Holder {
    val <!NonBooleanPropertyPrefixedWithIs!>isObject<!>: Int = 1
}

enum class Mode {
    A {
        val <!NonBooleanPropertyPrefixedWithIs!>isEntry<!>: Int = 1
    },
    B,
}

val <!NonBooleanPropertyPrefixedWithIs!>isTopLevel<!>: String = "top"

val String.<!NonBooleanPropertyPrefixedWithIs!>isExtension<!>: Int
    get() = length

// A type argument does not make the type a type parameter.
class Box<T>(value: T) {
    val <!NonBooleanPropertyPrefixedWithIs!>isValues<!>: List<T> = listOf(value)

    // A type parameter is unknown as the property's type, not as a return type.
    val <!NonBooleanPropertyPrefixedWithIs!>isSupplier<!>: () -> T = { value }
}

// Nothing declared explicitly is reported like any other declared type.
abstract class Declared {
    abstract val <!NonBooleanPropertyPrefixedWithIs!>isNothing<!>: Nothing
}

val anonymous = object {
    val <!NonBooleanPropertyPrefixedWithIs!>isInAnonymous<!>: String = "anon"
}

fun localClass(): Int {
    class Local {
        val <!NonBooleanPropertyPrefixedWithIs!>isInLocalClass<!>: Int = 1
    }
    return Local().isInLocalClass
}
