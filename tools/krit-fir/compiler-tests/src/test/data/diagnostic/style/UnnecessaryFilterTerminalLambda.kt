// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 10, 12
// Precision: a terminal that already takes a predicate. Go counts only the
// parenthesized arguments, so it reports these and suggests `.first { a }`,
// dropping the terminal's own predicate; the message is false of the code.
package test

fun bothPredicates(list: List<Int>): Int = list.filter { it > 0 }.first { it < 10 }

fun bothPredicatesCount(list: List<Int>): Int = list.filter { it > 0 }.count { it % 2 == 0 }

fun bothPredicatesAny(list: List<Int>): Boolean = list.filter { it > 0 }.any() { it > 5 }
