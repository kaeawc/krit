// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24
// Dispatchers.Main is not in InjectDispatcher's default dispatcherNames:
// config/default-krit.yml lists IO, Default, and Unconfined, while the Go
// rule's struct default also lists Main. The go-lines header is computed with
// the shipped config, as krit runs. Neither Go nor FIR reports Main itself.
// Where Main comes before a hardcoded IO in one call, the config decides Go's
// verdict: Go reads only the first listed dispatcher argument, so with Main
// in the list it stops at Main and reports nothing, and with the shipped list
// it reaches IO and reports it, as FIR does.
package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Presenter {
    suspend fun render(): String {
        return withContext(Dispatchers.Main) { "ui" }
    }

    fun handOff(ui: CoroutineDispatcher, work: CoroutineDispatcher): Int = ui.hashCode() + work.hashCode()

    fun both(): Int = handOff(Dispatchers.Main, <!InjectDispatcher!>Dispatchers.IO<!>)
}
