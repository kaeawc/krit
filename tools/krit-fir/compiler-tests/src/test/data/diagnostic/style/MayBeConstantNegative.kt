// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33
// Negative: declarations that cannot be, or already are, `const val`.
package test.maybeconstantnegative

import kotlin.jvm.JvmField
import kotlin.jvm.JvmStatic
import kotlin.properties.Delegates

const val ALREADY = "const"
var MUTABLE = "mutable"
val NULLABLE: String? = "nullable"
val ANY_TYPED: Any = "any"
val LIST_VALUE = listOf(1, 2)
val CALL_VALUE = "a".uppercase()
val CUSTOM_GETTER: Int
    get() = 3
val DELEGATED by lazy { 4 }
val DELEGATED_VAR: Int by Delegates.notNull()
val EXPLICIT_TYPE_ANY: Number = 5

fun compute(): Int = 9

val FROM_FUNCTION = compute()
val NOT_A_CONSTANT_REFERENCE = compute() + 1

object Annotated {
    @JvmField
    val FIELD = "field"

    // Go reports this, but the compiler rejects `@JvmStatic` on a `const`, so the suggested edit would not build.
    @JvmStatic
    val STATIC = "static"

    val CALCULATED = compute()
}

open class Base {
    open val OPEN_VALUE = 1
    val MEMBER = "member in a class body"

    fun local() {
        val LOCAL = 1
        println(LOCAL)
    }
}

class Derived : Base() {
    override val OPEN_VALUE = 2
}

object Anonymous {
    val holder = object {
        val INNER = 5
    }
}

fun usesAnonymous() = object {
    val INNER = "x"
}
