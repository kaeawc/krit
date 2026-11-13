// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// Recall: shapes Go cannot name or skips by text, where the resolved call and type decide.
package test

class Holder(val label: String, val rows: List<String>)

// Go lists the receiver types it knows by name, and ArrayList and Array are not among them.
fun arrayList(x: ArrayList<String>): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun array(x: Array<String>): Boolean = x.<!UselessCallOnNotNull!>isNullOrEmpty<!>()

fun hashMap(x: HashMap<String, Int>): Map<String, Int> = x.<!UselessCallOnNotNull!>orEmpty<!>()

// Go skips a receiver whose text contains a safe call, even inside an argument.
fun safeCallInsideArgument(h: Holder?, s: String): String = s.replace(h?.label ?: "", "").<!UselessCallOnNotNull!>orEmpty<!>()

// A safe call on a receiver that cannot be null: Go reports it as well.
fun safeCallOnNonNull(x: String): String? = x?.<!UselessCallOnNotNull!>orEmpty<!>()

// Go skips a navigation argument.
fun navigationArgument(h: Holder): List<String> = <!UselessCallOnNotNull!>listOfNotNull<!>(h.label)

fun navigationArguments(h: Holder): Set<String> = <!UselessCallOnNotNull!>setOfNotNull<!>(h.label, h.rows.first())

// Go skips an argument whose text names a stdlib `...OrNull` call, even when the result cannot be null.
fun orNullByNameOnly(h: Holder): List<Int> = <!UselessCallOnNotNull!>listOfNotNull<!>(h.rows.size.toString().length)

// Go reads only a bare callee name.
fun qualified(a: String): List<String> = kotlin.collections.<!UselessCallOnNotNull!>listOfNotNull<!>(a)

// Go requires a bare identifier to be declared in this file, so a parameter of a lambda or a
// property of another class is possibly null for it.
fun lambdaParameter(items: List<String>): List<String> = items.flatMap { <!UselessCallOnNotNull!>listOfNotNull<!>(it) }
