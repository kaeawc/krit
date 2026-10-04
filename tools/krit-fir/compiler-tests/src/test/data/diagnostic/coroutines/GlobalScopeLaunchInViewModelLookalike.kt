// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 18
// Divergence: Go takes any receiver spelled GlobalScope. Here GlobalScope is a
// local object with its own launch/async, not kotlinx.coroutines.GlobalScope,
// so nothing launches a coroutine on the global scope. Go reports both calls;
// FIR is correct to drop them.
package test.lookalike

object GlobalScope {
    fun launch(block: () -> Unit) = block()

    fun <T> async(block: () -> T): T = block()
}

class UserViewModel {
    fun load() {
        GlobalScope.launch { }
        GlobalScope.async { 1 }
    }
}
