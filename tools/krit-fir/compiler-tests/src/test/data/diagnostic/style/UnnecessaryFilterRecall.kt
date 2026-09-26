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

fun arrayList(values: ArrayList<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.first()

fun hashSet(values: HashSet<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.count()

fun array(values: Array<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.first()

fun intArray(values: IntArray): Boolean = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.any()

fun string(value: String): Int = <!UnnecessaryFilter!>value<!>.filter { it.isDigit() }.count()

fun charSequence(value: CharSequence): Char? = <!UnnecessaryFilter!>value<!>.filter { it.isDigit() }.lastOrNull()

class Scores : ArrayList<Int>() {
    // Go resolves `this` to Scores.
    fun positive(): Int = <!UnnecessaryFilter!>this<!>.filter { it > 0 }.single()
}

fun listSubclass(scores: Scores): Int = <!UnnecessaryFilter!>scores<!>.filter { it > 0 }.single()

// kotlinx.coroutines.flow has first(predicate) and firstOrNull(predicate).
suspend fun flowFirst(values: Flow<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.first()

suspend fun flowFirstOrNull(values: Flow<Int>): Int? = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.firstOrNull()

// A parenthesized filter call is still the filter's result; Go needs the call
// as the direct receiver of the terminal.
fun parenthesized(list: List<Int>): Int = <!UnnecessaryFilter!>(<!>list.filter { it > 0 }).first()

// Empty parentheses before the trailing lambda: tree-sitter nests the `()` call
// inside a second call, so Go does not see a `filter` call there.
fun emptyParens(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter() { it > 0 }.first()
