// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An apply body that reaches an extension function (launch, async) through the implicit receiver counts as one receiver reference. Markers sit where detekt 1.23.8 (type resolution) reports; the Scope stubs stand in for a coroutine scope. Go has no types, so it reports none of these.
package test

interface Scope
interface Job
class Chan { operator fun iterator(): Iterator<Int> = listOf(1).iterator() }

fun Scope.launch(block: suspend Scope.() -> Unit): Job = TODO()
fun <T> Scope.async(block: suspend Scope.() -> T): Job = TODO()
suspend fun pause() {}
val Scope.active: Boolean get() = true

class Svc(private val scope: Scope, private val ch: Chan) {
    fun helper() {}

    init { scope.<!UnnecessaryApply!>apply<!> { launch { helper() } } } // C01 launch resolves through the apply receiver
    init { scope.apply { launch { helper() }; launch { helper() } } } // C02 two statements
    init { scope.<!UnnecessaryApply!>apply<!> { launch { for (k in ch) { launch { pause() } } } } } // C03 nested launch binds to the outer launch
    init { scope.<!UnnecessaryApply!>apply<!> { this.launch { helper() } } } // C04 explicit this
    init { var job: Job? = null; scope.apply { job = launch { helper() } } } // C05 assignment target is a local
    init { scope.<!UnnecessaryApply!>apply<!> { async { 1 } } } // C06 async value unused
    init { scope.<!UnnecessaryApply!>apply<!> { launch { scope.launch { helper() } } } } // C07 nested explicit scope
    init { scope.apply { scope.launch { helper() } } } // C08 explicit receiver only
    init { scope.<!UnnecessaryApply!>apply<!> { launch { println(active) } } } // C09 launch plus a nested receiver property
    init { scope.<!UnnecessaryApply!>apply<!> { launch { pause() } } } // C10 suspend call in the launched body
    fun value() { val x = scope.apply { launch { helper() } } } // C11 apply value is read
    suspend fun suspending() { scope.<!UnnecessaryApply!>apply<!> { launch { pause() } } } // C12 inside a suspend function
}
