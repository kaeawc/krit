// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 9, 11, 13, 15, 17, 19, 21, 23, 26, 28, 30, 32, 34, 36, 38, 41, 45, 51, 58, 62, 64, 68, 72, 76, 78, 81, 83, 87, 91, 95, 96, 99, 101, 103, 107
// Positives: a stdlib filter with a trailing lambda followed by a
// no-argument terminal that has a predicate overload.
package test

fun first(list: List<Int>): Int = list.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

fun firstOrNull(list: List<Int>): Int? = list.filter { it > 0 }.<!UnnecessaryFilter!>firstOrNull<!>()

fun last(list: List<Int>): Int = list.filter { it > 0 }.<!UnnecessaryFilter!>last<!>()

fun lastOrNull(list: List<Int>): Int? = list.filter { it > 0 }.<!UnnecessaryFilter!>lastOrNull<!>()

fun single(list: List<Int>): Int = list.filter { it > 0 }.<!UnnecessaryFilter!>single<!>()

fun singleOrNull(list: List<Int>): Int? = list.filter { it > 0 }.<!UnnecessaryFilter!>singleOrNull<!>()

fun count(list: List<Int>): Int = list.filter { it > 0 }.<!UnnecessaryFilter!>count<!>()

fun any(list: List<Int>): Boolean = list.filter { it > 0 }.<!UnnecessaryFilter!>any<!>()

fun none(list: List<Int>): Boolean = list.filter { it > 0 }.<!UnnecessaryFilter!>none<!>()

// Other receivers Go accepts by name.
fun set(values: Set<String>): String = values.filter { it.isNotEmpty() }.<!UnnecessaryFilter!>first<!>()

fun mutableList(values: MutableList<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

fun collection(values: Collection<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>count<!>()

fun iterable(values: Iterable<Int>): Boolean = values.filter { it > 0 }.<!UnnecessaryFilter!>any<!>()

fun sequence(values: Sequence<Int>): Int = values.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

fun map(values: Map<String, Int>): Int = values.filter { it.value > 0 }.<!UnnecessaryFilter!>count<!>()

fun mapNone(values: Map<String, Int>): Boolean = values.filter { it.value > 0 }.<!UnnecessaryFilter!>none<!>()

// A call or a property chain as the receiver.
fun callReceiver(): Int = listOf(1, 2, 3).filter { it > 1 }.<!UnnecessaryFilter!>first<!>()

class Holder(val items: List<Int>)

fun propertyChain(holder: Holder): Int = holder.items.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

// The finding sits on the terminal's name, as in Go, so a split chain reports on the terminal's line.
fun multiLine(list: List<Int>): Int {
    return list
        .filter { it > 0 }
        .<!UnnecessaryFilter!>first<!>()
}

fun multiLineLambda(list: List<Int>): Int? {
    return list.filter {
        val doubled = it * 2
        doubled > 4
    }.<!UnnecessaryFilter!>firstOrNull<!>()
}

// Safe calls on either link.
fun safeCalls(list: List<Int>?): Int? = list?.filter { it > 0 }?.<!UnnecessaryFilter!>firstOrNull<!>()

fun safeFilter(list: List<Int>?): Int? = list?.filter { it > 0 }?.<!UnnecessaryFilter!>count<!>()

// An implicit receiver.
fun implicitReceiver(list: List<Int>): Int = with(list) {
    filter { it > 0 }.<!UnnecessaryFilter!>first<!>()
}

class Numbers : ArrayList<Int>() {
    fun firstPositive(): Int = filter { it > 0 }.<!UnnecessaryFilter!>first<!>()
}

// A labeled lambda, an explicit parameter.
fun labeled(list: List<Int>): Int = list.filter pos@{ return@pos it > 0 }.<!UnnecessaryFilter!>first<!>()

fun namedParameter(list: List<Int>): Int = list.filter { value -> value > 0 }.<!UnnecessaryFilter!>first<!>()

// Nested in a larger expression, and a chain inside a lambda.
fun nested(list: List<Int>): String = "first=" + list.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()

fun insideLambda(lists: List<List<Int>>): List<Int> = lists.map { it.filter { v -> v > 0 }.<!UnnecessaryFilter!>count<!>() }

// A member of an object expression.
val anonymous = object {
    fun pick(list: List<Int>): Int = list.filter { it > 0 }.<!UnnecessaryFilter!>first<!>()
}

// The chain continues after the terminal.
fun continued(list: List<String>): Char = list.filter { it.isNotEmpty() }.<!UnnecessaryFilter!>first<!>().first()

// Two chains in one expression.
fun twoChains(list: List<Int>): Int =
    list.filter { it > 0 }.<!UnnecessaryFilter!>count<!>() +
        list.filter { it > 1 }.<!UnnecessaryFilter!>count<!>()

// detekt also reports the emptiness checks and the size property.
fun isEmptyCheck(list: List<Int>): Boolean = list.filter { it > 0 }.<!UnnecessaryFilter!>isEmpty<!>()

fun isNotEmptyCheck(list: List<Int>): Boolean = list.filter { it > 0 }.<!UnnecessaryFilter!>isNotEmpty<!>()

fun sizeCheck(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.size

// A split `.size` chain reports on the first line of the chain, as in Go.
fun multiLineSize(list: List<Int>): Int =
    <!UnnecessaryFilter!>list<!>
        .filter { it > 0 }
        .size
