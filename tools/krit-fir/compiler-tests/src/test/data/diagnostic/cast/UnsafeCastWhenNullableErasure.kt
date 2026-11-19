// RENDER_DIAGNOSTICS_FULL_TEXT
// The JVM checks only the erased cast target. An unchecked cast whose operand
// already fits the erasure cannot throw, and `as?` would not return null, so
// only casts that can fail the erased check trigger UnsafeCastWhenNullable.
package test

interface Base

fun <T> unboundedTypeParameter(x: Any): T? = x as T?

fun <T : CharSequence> boundFits(x: String): T? = x as T?

fun <T : CharSequence> boundCanFail(x: Any): T? = x <!UnsafeCastWhenNullable!>as<!> T?

fun <T : Base, U : T> boundChain(x: Base): U? = x as U?

inline fun <reified T> reifiedTypeParameter(x: Any): T? = x <!UnsafeCastWhenNullable!>as<!> T?

fun listArgument(x: List<Any>): List<String>? = x as List<String>?

fun listCanFail(x: Collection<String>): List<String>? = x <!UnsafeCastWhenNullable!>as<!> List<String>?

fun arrayElementIsChecked(x: Array<Any>): Array<String>? = x <!UnsafeCastWhenNullable!>as<!> Array<String>?

class Store {
    private val values = mutableMapOf<String, Any?>()
    fun <T> get(key: String): T? = values[key] as T?
    fun string(key: String): String? = values[key] <!UnsafeCastWhenNullable!>as<!> String?
}

fun localClassArgument(x: List<Any>): Any? {
    class Holder<V>
    val h: Any = Holder<Int>()
    val unchecked = x as List<Holder<String>>?
    return h <!UnsafeCastWhenNullable!>as<!> Holder<String>?
}
