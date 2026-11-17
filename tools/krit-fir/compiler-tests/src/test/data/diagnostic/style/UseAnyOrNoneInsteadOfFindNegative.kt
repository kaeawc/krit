// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: already any / none, no predicate,
// identity comparisons, comparisons with something other than `null`, and a
// find result that is not compared directly.
package test

fun alreadyAny(list: List<Int>): Boolean = list.any { it > 0 }

fun alreadyNone(list: List<Int>): Boolean = list.none { it > 0 }

// No predicate: `firstOrNull()` checks emptiness, not a predicate.
fun noPredicate(list: List<Int>): Boolean = list.firstOrNull() != null

fun lastNoPredicate(list: List<Int>): Boolean = list.lastOrNull() == null

// Identity comparisons: Go matches only `==` and `!=`.
fun identity(list: List<Int>): Boolean = list.find { it > 0 } !== null

fun identityEquals(list: List<Int>): Boolean = list.find { it > 0 } === null

fun notNull(list: List<Int>): Boolean = list.find { it > 0 } != 0

fun twoCalls(list: List<Int>): Boolean = list.find { it > 0 } == list.find { it < 0 }

fun elvis(list: List<Int>): Int = list.find { it > 0 } ?: 0

fun viaLocal(list: List<Int>): Boolean {
    val found = list.find { it > 0 }
    return found != null
}

fun whenSubject(list: List<Int>): String = when (list.find { it > 0 }) {
    null -> "none"
    else -> "some"
}

fun otherFunctions(list: List<Int>): Boolean =
    list.singleOrNull { it > 0 } != null || list.indexOfFirst { it > 0 } != -1

fun regex(text: String): Boolean = Regex("[0-9]").find(text) != null

// The comparison implements the replacement itself: `any { }` here would call itself.
class Bag(private val items: List<Int>) {
    fun find(predicate: (Int) -> Boolean): Int? = items.find(predicate)
    fun any(predicate: (Int) -> Boolean): Boolean = find(predicate) != null
    fun none(predicate: (Int) -> Boolean): Boolean = find { predicate(it) } == null
}
