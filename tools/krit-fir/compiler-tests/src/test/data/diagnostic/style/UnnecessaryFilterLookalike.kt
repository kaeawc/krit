// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 26, 37, 40, 42, 48, 53, 60
// Precision: chains whose filter or terminal is not the stdlib (or Flow)
// function with a predicate overload, so the `.first { pred }` form the
// message suggests does not exist or does something else. Go matches the
// names and reports each chain below whose receiver it cannot resolve to a
// known type.
package test

import java.util.stream.Stream
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.single

// A query builder with its own filter and terminals.
class Query {
    fun filter(predicate: (String) -> Boolean): Query = this
    fun first(): String = ""
    fun count(): Int = 0
}

fun query(): Query = Query()

fun builderChain(): String = query().filter { it.isNotEmpty() }.first()

fun builderCount(): Int = query().filter { it.isNotEmpty() }.count()

// A filter extension on a user type.
class Box<T>(val items: List<T>)

fun <T> Box<T>.filter(predicate: (T) -> Boolean): Box<T> = Box(items.filter(predicate))

fun <T> Box<T>.any(): Boolean = items.isNotEmpty()

fun makeBox(): Box<Int> = Box(listOf(1))

fun boxChain(): Boolean = makeBox().filter { it > 0 }.any()

// A Java Stream: filter is a member and Stream has no count(predicate).
fun streamCount(list: List<Int>): Long = list.stream().filter { it > 0 }.count()

fun streamOf(): Long = Stream.of(1, 2).filter { it > 0 }.count()

// A Flow has no predicate overload of single: `numbers().filter { }.single()`
// cannot become `numbers().single { }`.
fun numbers(): Flow<Int> = TODO()

suspend fun flowSingle(): Int = numbers().filter { it > 0 }.single()

// A local function named filter.
fun localFilter(list: List<Int>): Int {
    fun filter(predicate: (Int) -> Boolean): List<Int> = list.filter(predicate)
    return filter { it > 0 }.first()
}

// A member named filter on a class that is not a collection.
class Repository {
    fun filter(predicate: (Int) -> Boolean): List<Int> = emptyList()

    fun firstMatch(): Int = filter { it > 0 }.first()
}
