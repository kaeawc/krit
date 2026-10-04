// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Positive: hardcoded Dispatchers.IO inside a class member (injectable via the
// constructor) should trigger InjectDispatcher
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Repository {
    suspend fun loadData(): String {
        return withContext(<!InjectDispatcher!>Dispatchers.IO<!>) { "data" }
    }
}
