// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 26
// Dispatchers.Unconfined is a deliberate choice (for example with
// CoroutineStart.UNDISPATCHED), not a swappable default, so it is not reported.
// IO and Default in the same class still are.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class Loader(private val scope: CoroutineScope) {
    fun startNow() {
        scope.launch(Dispatchers.Unconfined, start = CoroutineStart.UNDISPATCHED) { }
    }

    suspend fun direct(): String = withContext(Dispatchers.Unconfined) { "data" }

    suspend fun combined(): String = withContext(Dispatchers.Unconfined + Dispatchers.Main) { "data" }

    suspend fun background(): String = withContext(<!InjectDispatcher!>Dispatchers.IO<!>) { "data" }

    fun compute() {
        scope.launch(<!InjectDispatcher!>Dispatchers.Default<!>) { }
    }
}
