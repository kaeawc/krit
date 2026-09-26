// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone.
package test

// Already the predicate form.
fun predicateForms(list: List<Int>): Boolean =
    list.first { it > 0 } > 0 && list.count { it > 0 } > 0 && list.any { it > 0 }

// filter followed by something that is not a terminal with a predicate overload.
fun filterThenMap(list: List<Int>): List<Int> = list.filter { it > 0 }.map { it * 2 }

fun filterThenSize(list: List<Int>): Int = list.filter { it > 0 }.size

fun filterThenIsEmpty(list: List<Int>): Boolean = list.filter { it > 0 }.isEmpty()

fun filterThenMax(list: List<Int>): Int = list.filter { it > 0 }.max()

// A terminal with an argument.
fun elementAt(list: List<Int>): Int = list.filter { it > 0 }.elementAt(0)

// Other filtering functions.
fun filterNot(list: List<Int>): Int = list.filterNot { it > 0 }.first()

fun filterIndexed(list: List<Int>): Int = list.filterIndexed { i, v -> i > 0 && v > 0 }.first()

fun filterIsInstance(list: List<Any>): String = list.filterIsInstance<String>().first()

fun filterNotNull(list: List<Int?>): Int = list.filterNotNull().first()

// The predicate is not a trailing lambda.
fun functionReference(list: List<Int>): Int = list.filter(::isPositive).first()

fun parenthesizedLambda(list: List<Int>): Int = list.filter({ it > 0 }).first()

fun predicateValue(list: List<Int>, predicate: (Int) -> Boolean): Int = list.filter(predicate).first()

fun isPositive(value: Int): Boolean = value > 0

// The filtered list is stored first.
fun storedFirst(list: List<Int>): Int {
    val positives = list.filter { it > 0 }
    return positives.first()
}

// `!!` between the links.
fun notNullAsserted(list: List<Int>?): Int = list?.filter { it > 0 }!!.first()

// The terminal is not called on the filter result.
fun terminalInsideLambda(list: List<List<Int>>): List<List<Int>> = list.filter { it.first() > 0 }
