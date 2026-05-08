package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun loadData(): String {
    return withContext(Dispatchers.IO) {
        "result"
    }
}
