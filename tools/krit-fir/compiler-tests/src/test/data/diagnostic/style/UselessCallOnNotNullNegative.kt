// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: the receiver or an argument can be null, or the call is not the stdlib's.
package test

class Holder(val label: String?, val rows: List<String>?)

fun nullable(x: String?): String = x.orEmpty()

fun nullableList(x: List<String>?): List<String> = x.orEmpty()

fun nullableCollection(x: Collection<String>?): Boolean = x.isNullOrEmpty()

fun nullableCharSequence(x: CharSequence?): Boolean = x.isNullOrBlank()

fun property(h: Holder): String = h.label.orEmpty()

fun propertyList(h: Holder): List<String> = h.rows.orEmpty()

fun safeCallResult(x: Holder?): String = x?.label.orEmpty()

fun mapLookup(m: Map<String, String>): String = m["k"].orEmpty()

fun firstOrNull(l: List<String>): Boolean = l.firstOrNull().isNullOrEmpty()

// A platform type can be null.
fun platform(): Boolean = System.getProperty("key").isNullOrEmpty()

// A smart cast does not change the declared type, as in detekt.
fun smartCast(x: String?): String {
    if (x != null) return x.orEmpty()
    return ""
}

// A safe call on a nullable receiver.
fun safeCall(x: String?): String? = x?.orEmpty()

fun nullArgument(): List<String> = listOfNotNull(null)

fun nullableArgument(a: String?): List<String> = listOfNotNull(a)

fun mixedArguments(a: String, b: String?): List<String> = listOfNotNull(a, b)

fun navigationArgument(h: Holder): List<String> = listOfNotNull(h.label)

fun callArgument(l: List<String>): List<String> = listOfNotNull(l.firstOrNull())

fun spreadArgument(a: Array<String?>): List<String> = listOfNotNull(*a)

fun noArguments(): List<String> = listOfNotNull()

fun platformArgument(): List<String> = listOfNotNull(System.getProperty("key"))

fun smartCastArgument(a: String?): List<String> {
    if (a == null) return emptyList()
    return listOfNotNull(a)
}

fun plainList(a: String): List<String> = listOf(a)
