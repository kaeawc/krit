// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 29, 31, 33, 35, 43, 56, 58
// Lookalikes: functions named find / firstOrNull / lastOrNull on receivers
// with no `any` / `none` taking a predicate, as a member or as an extension.
// Go matches the callee name alone and reports the explicit-receiver
// comparisons below, but the message is false: there is no `any {}` or
// `none {}` to use instead, so the checker drops them. (A find on a receiver
// that does have any / none is kept: UseAnyOrNoneInsteadOfFindUserCounterpart.)
package test

class Repository {
    fun find(predicate: (Int) -> Boolean): Int? = null
}

object Finder {
    fun firstOrNull(predicate: (String) -> Boolean): String? = null
}

class Stream<T> {
    fun lastOrNull(predicate: (T) -> Boolean): T? = null
}

class Box

fun Box.find(predicate: (Int) -> Boolean): Int? = null

fun member(repository: Repository): Boolean = repository.find { it > 0 } != null

fun objectMember(): Boolean = Finder.firstOrNull { it.isEmpty() } == null

fun genericMember(stream: Stream<Int>): Boolean = stream.lastOrNull { it > 0 } != null

fun localExtension(box: Box): Boolean = box.find { it > 0 } != null

fun safeCallMember(repository: Repository?): Boolean = repository?.find { it > 0 } != null

// Implicit receiver: Go needs an explicit receiver, so it skips this too.
fun implicitMember(repository: Repository): Boolean = with(repository) { find { it > 0 } != null }

val anonymousFinder = object {
    fun find(predicate: (Int) -> Boolean): Int? = null

    fun viaThis(): Boolean = this.find { it > 0 } != null
}

// any / none without a predicate do not count: `counter.any { ... }` does not
// compile.
class Counter {
    fun find(predicate: (Int) -> Boolean): Int? = null
    fun any(): Boolean = false
    fun none(): Boolean = true
}

fun Counter.any(threshold: Int): Boolean = threshold > 0

fun noPredicateCounterpart(counter: Counter): Boolean = counter.find { it > 0 } != null

fun noPredicateCounterpartIsNull(counter: Counter): Boolean = counter.find { it > 0 } == null
