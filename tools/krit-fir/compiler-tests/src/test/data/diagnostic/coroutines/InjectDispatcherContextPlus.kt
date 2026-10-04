// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 31, 34, 37, 40
// A dispatcher combined with other context elements through `+` belongs to
// the call the combined context is passed to, whichever side of the `+` it is
// on. `+` is itself a call (CoroutineContext.plus), so before this was pinned
// FIR judged `plus` as the host: it reported the right-hand operand
// everywhere, including inside the exempt CoroutineScope(...) constructor,
// and never looked at the left-hand one, while Go read only a lone
// `Dispatchers.X` argument and reported none of these.
package test

import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.CoroutineName
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class ImageCache(private val external: CoroutineScope) {
    // Scope construction is an idiomatic dispatcher host, combined or not.
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val named = CoroutineScope(Dispatchers.Default + CoroutineName("cache"))

    // Not passed to a call: the rule covers dispatchers passed as arguments.
    private val context = SupervisorJob() + Dispatchers.IO

    private val handler = CoroutineExceptionHandler { _, _ -> }

    suspend fun rightOperand(): String =
        withContext(CoroutineName("load") + <!InjectDispatcher!>Dispatchers.IO<!>) { "data" }

    suspend fun leftOperand(): String =
        withContext(<!InjectDispatcher!>Dispatchers.IO<!> + CoroutineName("load")) { "data" }

    suspend fun parenthesized(): String =
        withContext((handler + <!InjectDispatcher!>Dispatchers.Default<!>) + CoroutineName("load")) { "data" }

    fun launched() {
        external.launch(handler + <!InjectDispatcher!>Dispatchers.Unconfined<!>) { }
    }

    // Main cannot be injected for tests.
    suspend fun mainOperand(): String =
        withContext(Dispatchers.Main + CoroutineName("ui")) { "ui" }

    fun use(): Int = scope.hashCode() + named.hashCode() + context.hashCode()
}

// No class to inject into.
suspend fun topLevel(): String = withContext(CoroutineName("load") + Dispatchers.IO) { "data" }
