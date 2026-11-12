// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 33, 35, 37, 50, 53, 56, 58, 60
// Positives: a null-tolerant stdlib call on a receiver that cannot be null, and
// `listOfNotNull` / `setOfNotNull` whose arguments cannot be null.
package test

class Box(val name: String, val items: List<String>)

fun literal(): String = "hello".<!UselessCallOnNotNull!>orEmpty<!>()

fun string(x: String): String = x.<!UselessCallOnNotNull!>orEmpty<!>()

fun list(x: List<String>): List<String> = x.<!UselessCallOnNotNull!>orEmpty<!>()

fun set(x: Set<Int>): Set<Int> = x.<!UselessCallOnNotNull!>orEmpty<!>()

fun map(x: Map<String, Int>): Map<String, Int> = x.<!UselessCallOnNotNull!>orEmpty<!>()

fun sequence(x: Sequence<Int>): Sequence<Int> = x.<!UselessCallOnNotNull!>orEmpty<!>()

fun mutableList(x: MutableList<String>): List<String> = x.<!UselessCallOnNotNull!>orEmpty<!>()

fun factory(): List<Int> = listOf(1, 2).<!UselessCallOnNotNull!>orEmpty<!>()

fun emptyOfFactory(): Set<Int> = emptySet<Int>().<!UselessCallOnNotNull!>orEmpty<!>()

fun nullOrEmptyString(x: String): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun nullOrBlankString(x: String): Boolean = x.<!UselessCallOnNotNull!>isNullOrBlank<!>()

fun nullOrEmptyCharSequence(x: CharSequence): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun nullOrEmptyList(x: List<String>): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun nullOrEmptyCollection(x: Collection<String>): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun nullOrEmptyMap(x: Map<String, Int>): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

// A property read: the member's declared type decides.
fun property(box: Box): String = box.name.<!UselessCallOnNotNull!>orEmpty<!>()

fun propertyList(box: Box): Boolean = box.items.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

// The `!!` operator leaves a non-null value.
fun bangBang(x: String?): String = x!!.<!UselessCallOnNotNull!>orEmpty<!>()

// A multi-line chain reports on the line of the method name.
fun chain(x: List<String>): List<String> = x
    .map { it.trim() }
    .<!UselessCallOnNotNull!>orEmpty<!>()

fun resolvedArgument(value: Box) {
    println(<!UselessCallOnNotNull!>listOfNotNull<!>(value))
}

fun manyArguments(a: String, b: String) = <!UselessCallOnNotNull!>listOfNotNull<!>(a, b, "c")

fun setArguments(a: Int) = <!UselessCallOnNotNull!>setOfNotNull<!>(a, 2)

fun literalArgument() = <!UselessCallOnNotNull!>listOfNotNull<!>("x")
