// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

suspend fun loadOnce() {
    withContext(Dispatchers.IO) {
        fetch()
    }
}

suspend fun loadThenUi() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.Main) {
            fetch()
        }
    }
}

class Repo(private val io: CoroutineDispatcher) {
    suspend fun load() {
        withContext(io) {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}

suspend fun loadInLaunch(scope: CoroutineScope) {
    withContext(Dispatchers.IO) {
        scope.launch {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}

suspend fun loadInScope() {
    withContext(Dispatchers.IO) {
        coroutineScope {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}

suspend fun loadViaHelper() {
    withContext(Dispatchers.IO) {
        suspend fun helper() {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
        helper()
    }
}

// Only the nearest withContext decides: IO inside Main inside IO is a real switch.
suspend fun switchBack() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.Main) {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}

// A sibling call is not an enclosing one.
suspend fun sequential() {
    withContext(Dispatchers.IO) { fetch() }
    withContext(Dispatchers.IO) { fetch() }
}

// A nested anonymous object is a new scope.
suspend fun inObject() {
    withContext(Dispatchers.IO) {
        object : Runnable {
            override fun run() {}
        }
        fetch()
    }
}

suspend fun fetch() {}
