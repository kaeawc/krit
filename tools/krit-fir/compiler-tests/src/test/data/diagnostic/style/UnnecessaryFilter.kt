// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 9, 11, 13, 15, 17, 19, 21, 23, 26, 28, 30, 32, 34, 36, 38, 41, 45, 49, 55, 62, 64, 68, 72, 76, 78, 81, 83, 87, 91, 95, 96
// Positives: a stdlib filter with a trailing lambda followed by a
// no-argument terminal that has a predicate overload.
package test

fun first(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.first()

fun firstOrNull(list: List<Int>): Int? = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.firstOrNull()

fun last(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.last()

fun lastOrNull(list: List<Int>): Int? = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.lastOrNull()

fun single(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.single()

fun singleOrNull(list: List<Int>): Int? = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.singleOrNull()

fun count(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.count()

fun any(list: List<Int>): Boolean = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.any()

fun none(list: List<Int>): Boolean = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.none()

// Other receivers Go accepts by name.
fun set(values: Set<String>): String = <!UnnecessaryFilter!>values<!>.filter { it.isNotEmpty() }.first()

fun mutableList(values: MutableList<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.first()

fun collection(values: Collection<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.count()

fun iterable(values: Iterable<Int>): Boolean = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.any()

fun sequence(values: Sequence<Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it > 0 }.first()

fun map(values: Map<String, Int>): Int = <!UnnecessaryFilter!>values<!>.filter { it.value > 0 }.count()

fun mapNone(values: Map<String, Int>): Boolean = <!UnnecessaryFilter!>values<!>.filter { it.value > 0 }.none()

// A call or a property chain as the receiver.
fun callReceiver(): Int = <!UnnecessaryFilter!>listOf<!>(1, 2, 3).filter { it > 1 }.first()

class Holder(val items: List<Int>)

fun propertyChain(holder: Holder): Int = <!UnnecessaryFilter!>holder<!>.items.filter { it > 0 }.first()

// The finding sits on the first line of the chain, as in Go.
fun multiLine(list: List<Int>): Int {
    return <!UnnecessaryFilter!>list<!>
        .filter { it > 0 }
        .first()
}

fun multiLineLambda(list: List<Int>): Int? {
    return <!UnnecessaryFilter!>list<!>.filter {
        val doubled = it * 2
        doubled > 4
    }.firstOrNull()
}

// Safe calls on either link.
fun safeCalls(list: List<Int>?): Int? = <!UnnecessaryFilter!>list<!>?.filter { it > 0 }?.firstOrNull()

fun safeFilter(list: List<Int>?): Int? = <!UnnecessaryFilter!>list<!>?.filter { it > 0 }?.count()

// An implicit receiver.
fun implicitReceiver(list: List<Int>): Int = with(list) {
    <!UnnecessaryFilter!>filter<!> { it > 0 }.first()
}

class Numbers : ArrayList<Int>() {
    fun firstPositive(): Int = <!UnnecessaryFilter!>filter<!> { it > 0 }.first()
}

// A labeled lambda, an explicit parameter.
fun labeled(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter pos@{ return@pos it > 0 }.first()

fun namedParameter(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { value -> value > 0 }.first()

// Nested in a larger expression, and a chain inside a lambda.
fun nested(list: List<Int>): String = "first=" + <!UnnecessaryFilter!>list<!>.filter { it > 0 }.first()

fun insideLambda(lists: List<List<Int>>): List<Int> = lists.map { <!UnnecessaryFilter!>it<!>.filter { v -> v > 0 }.count() }

// A member of an object expression.
val anonymous = object {
    fun pick(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.first()
}

// The chain continues after the terminal.
fun continued(list: List<String>): Char = <!UnnecessaryFilter!>list<!>.filter { it.isNotEmpty() }.first().first()

// Two chains in one expression.
fun twoChains(list: List<Int>): Int =
    <!UnnecessaryFilter!>list<!>.filter { it > 0 }.count() +
        <!UnnecessaryFilter!>list<!>.filter { it > 1 }.count()
