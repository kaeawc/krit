// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 12, 13, 14, 20, 21, 26, 27, 28, 31, 34, 35, 38, 43, 49, 50, 52, 58, 62
// Positive: each stdlib factory called with no elements should use its empty
// counterpart. The finding sits on the callee's line, where Go reports the
// call expression.
package test

fun factories() {
    val a: List<String> = <!UseEmptyCounterpart!>listOf<!>()
    val b: List<String> = <!UseEmptyCounterpart!>listOfNotNull<!>()
    val c: Set<Int> = <!UseEmptyCounterpart!>setOf<!>()
    val d: Map<String, Int> = <!UseEmptyCounterpart!>mapOf<!>()
    val e: Array<String> = <!UseEmptyCounterpart!>arrayOf<!>()
    val f: Sequence<Int> = <!UseEmptyCounterpart!>sequenceOf<!>()
    println(listOf(a, b, c, d, e, f))
}

// Explicit type arguments are not value arguments.
fun typeArguments() {
    val a = <!UseEmptyCounterpart!>listOf<!><String>()
    val b = <!UseEmptyCounterpart!>mapOf<!><String, Int>()
    println(listOf(a, b))
}

// A call used as a receiver, an argument, a return value or a default.
fun used(items: List<String> = <!UseEmptyCounterpart!>listOf<!>()): Int {
    println(<!UseEmptyCounterpart!>setOf<!><Int>().size + items.size)
    return <!UseEmptyCounterpart!>listOf<!><Int>().size
}

fun returned(): Map<String, Int> = <!UseEmptyCounterpart!>mapOf<!>()

class Holder {
    val names: List<String> = <!UseEmptyCounterpart!>listOf<!>()
    var tags = <!UseEmptyCounterpart!>setOf<!><String>()

    companion object {
        val EMPTY: Array<String> = <!UseEmptyCounterpart!>arrayOf<!>()
    }
}

object Registry {
    val items: Sequence<String> = <!UseEmptyCounterpart!>sequenceOf<!>()
}

// Nested scopes: lambdas, local functions and anonymous objects still call
// the stdlib factory.
fun nested() {
    val block = { <!UseEmptyCounterpart!>listOf<!><String>() }
    fun local(): Set<String> = <!UseEmptyCounterpart!>setOf<!>()
    val anon = object {
        val m: Map<Int, Int> = <!UseEmptyCounterpart!>mapOf<!>()
    }
    println(listOf(block(), local(), anon.m))
}

// A generic array factory in a reified inline function.
inline fun <reified T> emptyOf(): Array<T> = <!UseEmptyCounterpart!>arrayOf<!>()

// An empty argument list split over lines still reports on the callee's line.
fun multiline(): List<String> =
    <!UseEmptyCounterpart!>listOf<!>(
    )
