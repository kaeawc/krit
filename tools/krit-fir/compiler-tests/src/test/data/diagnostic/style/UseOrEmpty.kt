// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 32, 35, 39, 42, 44, 46, 48, 50, 53, 56, 59, 61, 63, 68, 74, 77, 82, 85
// Positives: an Elvis whose fallback is the stdlib empty value of the left
// side's type, which `.orEmpty()` returns.
package test

fun list(x: List<String>?): List<String> {
    return <!UseOrEmpty!>x ?: emptyList()<!>
}

fun set(x: Set<Int>?): Set<Int> = <!UseOrEmpty!>x ?: emptySet()<!>

fun map(x: Map<String, Int>?): Map<String, Int> = <!UseOrEmpty!>x ?: emptyMap()<!>

fun sequence(x: Sequence<Int>?): Sequence<Int> = <!UseOrEmpty!>x ?: emptySequence()<!>

fun string(x: String?): String = <!UseOrEmpty!>x ?: ""<!>

fun rawString(x: String?): String = <!UseOrEmpty!>x ?: """"""<!>

fun listOfEmpty(x: List<String>?): List<String> = <!UseOrEmpty!>x ?: listOf()<!>

fun setOfEmpty(x: Set<Int>?): Set<Int> = <!UseOrEmpty!>x ?: setOf()<!>

fun mapOfEmpty(x: Map<String, Int>?): Map<String, Int> = <!UseOrEmpty!>x ?: mapOf()<!>

fun arrayOfEmpty(x: Array<Int>?): Array<out Int> = <!UseOrEmpty!>x ?: arrayOf()<!>

fun sequenceOfEmpty(x: Sequence<Int>?): Sequence<Int> = <!UseOrEmpty!>x ?: sequenceOf()<!>

// A qualified stdlib call is still the fallback.
fun qualified(x: List<String>?): List<String> = <!UseOrEmpty!>x ?: kotlin.collections.emptyList()<!>

// Go only skips a fallback whose text starts with `emptyArray(`.
fun qualifiedEmptyArray(x: Array<Int>?): Array<out Int> = <!UseOrEmpty!>x ?: kotlin.emptyArray()<!>

// Go only skips a fallback whose text starts with `emptyArray(`, so it
// reports one with whitespace before the parentheses.
fun spacedEmptyArray(x: Array<Int>?): Array<out Int> = <!UseOrEmpty!>x ?: emptyArray ()<!>

// Collection subtypes and supertypes that `.orEmpty()` accepts.
fun mutable(x: MutableList<String>?): List<String> = <!UseOrEmpty!>x ?: emptyList()<!>

fun collection(x: Collection<String>?): Collection<String> = <!UseOrEmpty!>x ?: emptyList()<!>

fun crossCollection(x: Set<String>?): Collection<String> = <!UseOrEmpty!>x ?: emptyList()<!>

fun hashMap(x: HashMap<String, Int>?): Map<String, Int> = <!UseOrEmpty!>x ?: emptyMap()<!>

fun <T : List<String>> bounded(x: T?): List<String> = <!UseOrEmpty!>x ?: emptyList()<!>

// The JDK's empty collections are the same empty value.
fun jdkEmpty(x: List<String>?): List<String> = <!UseOrEmpty!>x ?: java.util.Collections.emptyList()<!>

// Java platform types.
fun platform(): String = <!UseOrEmpty!>System.getProperty("key") ?: ""<!>

// A map lookup, a call, and a nested Elvis on the left.
fun lookup(map: Map<String, List<Int>>): List<Int> = <!UseOrEmpty!>map["key"] ?: emptyList()<!>

fun call(f: () -> String?): String = <!UseOrEmpty!>f() ?: ""<!>

fun nested(a: String?, b: String?): String = <!UseOrEmpty!>a ?: b ?: ""<!>

// The finding is on the line where the Elvis starts.
fun multiLine(x: List<String>?): List<String> {
    val result =
        <!UseOrEmpty!>x<!>
            ?: emptyList()
    return result
}

class Holder(private val items: List<String>?) {
    fun get(): List<String> = <!UseOrEmpty!>items ?: emptyList()<!>

    companion object {
        fun of(x: String?): String = <!UseOrEmpty!>x ?: ""<!>
    }
}

val anonymous = object {
    fun get(x: String?): String = <!UseOrEmpty!>x ?: ""<!>
}

fun inLambda(values: List<String?>): List<String> = values.map { <!UseOrEmpty!>it ?: ""<!> }
