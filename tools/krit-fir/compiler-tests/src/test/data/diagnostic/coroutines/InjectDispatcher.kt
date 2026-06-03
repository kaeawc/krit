// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: hardcoded Dispatchers.IO inside a class member (injectable via the
// constructor) should trigger INJECT_DISPATCHER
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Repository {
    suspend fun loadData(): String {
        return withContext(<!INJECT_DISPATCHER!>Dispatchers.IO<!>) { "data" }
    }
}
