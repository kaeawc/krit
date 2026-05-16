package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

// Single withContext: no nesting.
suspend fun loadOnce() {
    withContext(Dispatchers.IO) {
        fetch()
    }
}

// Inner uses a different dispatcher than the outer.
suspend fun loadThenUi() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.Main) {
            updateUi()
        }
    }
}

// Injected/variable dispatcher: rule cannot prove symbolic equality.
class Repo(private val io: CoroutineDispatcher) {
    suspend fun load() {
        withContext(io) {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}

// Intervening coroutine builder (launch/coroutineScope) creates a new
// context — re-stating the dispatcher is intentional, not a no-op.
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

// Nested local function declaration is a fresh scope boundary.
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

// Comments and strings that look like withContext must not fire.
suspend fun lookalikes() {
    // withContext(Dispatchers.IO) { withContext(Dispatchers.IO) { } }
    val msg = "withContext(Dispatchers.IO) inside withContext(Dispatchers.IO)"
    withContext(Dispatchers.IO) {
        println(msg)
    }
}

fun updateUi() {}
suspend fun fetch() {}
