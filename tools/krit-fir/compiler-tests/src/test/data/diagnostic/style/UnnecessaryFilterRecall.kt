// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Recall: chains Go skips because it resolves the receiver's name to a type
// outside its list (List, MutableList, Collection, Iterable, Set, MutableSet,
// Sequence, Map, MutableMap), although the stdlib (or Flow) predicate overload
// the message suggests exists for each of them.
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.firstOrNull

fun arrayList(values: ArrayList<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

fun hashSet(values: HashSet<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>count<!>()

fun array(values: Array<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

fun intArray(values: IntArray): Boolean = values.filter { it > 0 }.<!UnnecessaryFilter!>any<!>()

fun string(value: String): Int = value.filter { it.isDigit() }.<!UnnecessaryFilter!>count<!>()

fun charSequence(value: CharSequence): Char? = value.filter { it.isDigit() }.<!UnnecessaryFilter!>lastOrNull<!>()

class Scores : ArrayList<Int>() {
    // Go resolves `this` to Scores.
    fun positive(): Int = this.filter { it > 0 }.<!UnnecessaryFilter!>single<!>()
}

fun listSubclass(scores: Scores): Int = scores.filter { it > 0 }.<!UnnecessaryFilter!>single<!>()

// kotlinx.coroutines.flow has first(predicate) and firstOrNull(predicate).
suspend fun flowFirst(values: Flow<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

suspend fun flowFirstOrNull(values: Flow<Int>): Int? = values.filter { it > 0 }.<!UnnecessaryFilter!>firstOrNull<!>()

// A parenthesized filter call is still the filter's result; Go needs the call
// as the direct receiver of the terminal.
fun parenthesized(list: List<Int>): Int = (list.filter { it > 0 }).<!UnnecessaryFilter!>first<!>()

// Empty parentheses before the trailing lambda: tree-sitter nests the `()` call
// inside a second call, so Go does not see a `filter` call there.
fun emptyParens(list: List<Int>): Int = list.filter() { it > 0 }.<!UnnecessaryFilter!>first<!>()
