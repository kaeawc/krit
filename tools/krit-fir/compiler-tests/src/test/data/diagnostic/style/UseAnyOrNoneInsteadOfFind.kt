// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 31, 33, 35, 37, 40, 43, 46, 48, 51, 55, 57, 61x2, 64, 69, 75, 77, 80, 85, 89, 93, 98, 101
// Positives: a stdlib find / firstOrNull / lastOrNull with a trailing
// predicate lambda, compared with `null` by `!=` or `==` on either side.
package test

fun findNotNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != null<!>

fun findIsNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } == null<!>

fun firstOrNullNotNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.firstOrNull { it > 0 } != null<!>

fun lastOrNullIsNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.lastOrNull { it > 0 } == null<!>

fun nullOnLeft(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>null != list.find { it > 0 }<!>

fun nullOnLeftEquals(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>null == list.lastOrNull { it > 0 }<!>

fun onSet(set: Set<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>set.find { it.isEmpty() } != null<!>

fun onIterable(items: Iterable<Long>): Boolean = <!UseAnyOrNoneInsteadOfFind!>items.firstOrNull { it > 0L } == null<!>

fun onArray(array: Array<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>array.find { it.isEmpty() } != null<!>

fun onIntArray(array: IntArray): Boolean = <!UseAnyOrNoneInsteadOfFind!>array.lastOrNull { it > 0 } != null<!>

fun onSequence(sequence: Sequence<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>sequence.firstOrNull { it > 0 } != null<!>

fun onString(text: String): Boolean = <!UseAnyOrNoneInsteadOfFind!>text.find { it.isDigit() } != null<!>

fun onMapEntries(map: Map<String, Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>map.entries.find { it.value > 0 } == null<!>

fun afterChain(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.filter { it > 0 }.map { it * 2 }.find { it > 10 } != null<!>

fun explicitTypeArgument(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find<Int> { it > 0 } != null<!>

fun labeledLambda(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find predicate@{ return@predicate it > 0 } != null<!>

// Safe call: Go reports it, and it is still a find compared with null.
fun safeCall(list: List<Int>?): Boolean = <!UseAnyOrNoneInsteadOfFind!>list?.find { it > 0 } != null<!>

// Elements that may be null: Go reports it (a judgment call the checker keeps).
fun nullableElements(list: List<Int?>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it != null && it > 0 } != null<!>

// Java collections resolve to the same stdlib extensions (platform types).
fun javaCollection(): Boolean = <!UseAnyOrNoneInsteadOfFind!>System.getenv().keys.find { it.startsWith("KRIT") } != null<!>

fun javaArrayList(list: java.util.ArrayList<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.firstOrNull { it.isEmpty() } == null<!>

fun inCondition(list: List<Int>): Int {
    if (<!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != null<!>) return 1
    return 0
}

fun inConjunction(list: List<Int>, flag: Boolean): Boolean = flag && <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } == null<!>

fun negated(list: List<Int>): Boolean = !(<!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } == null<!>)

// Two findings on one line: the outer and the inner comparison.
fun nested(groups: List<List<Int>>): Boolean =
    <!UseAnyOrNoneInsteadOfFind!>groups<!>.find { group -> group.find { it > 0 } != null } != null

fun multiLine(list: List<Int>): Boolean {
    return <!UseAnyOrNoneInsteadOfFind!>list<!>
        .find { it > 0 } != null
}

fun multiLineLambda(list: List<Int>): Boolean {
    return <!UseAnyOrNoneInsteadOfFind!>list.firstOrNull {<!>
        it > 0
    } != null
}

class Holder(private val values: List<Int>) {
    fun member(): Boolean = <!UseAnyOrNoneInsteadOfFind!>values.find { it > 0 } != null<!>

    fun viaThis(): Boolean = <!UseAnyOrNoneInsteadOfFind!>this.values.lastOrNull { it > 0 } != null<!>

    companion object {
        fun fromCompanion(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } == null<!>
    }
}

class Bag : ArrayList<Int>() {
    fun explicitThis(): Boolean = <!UseAnyOrNoneInsteadOfFind!>this.find { it > 0 } != null<!>
}

fun insideLambda(list: List<Int>): Boolean = run {
    <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != null<!>
}

fun localFunction(list: List<Int>): Boolean {
    fun inner(): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != null<!>
    return inner()
}

val anonymous = object {
    fun check(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != null<!>
}

val topLevel: Boolean = <!UseAnyOrNoneInsteadOfFind!>listOf(1, 2, 3).find { it > 2 } != null<!>
