// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 27, 29, 31, 33, 41
// Lookalikes: functions named find / firstOrNull / lastOrNull that are not the
// standard library's, so the receiver has no `any` / `none` to use instead.
// Go matches the callee name alone and reports the explicit-receiver
// comparisons below; the checker requires the stdlib function.
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
