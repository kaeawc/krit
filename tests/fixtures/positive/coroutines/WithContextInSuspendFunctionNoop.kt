package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun loadData() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.IO) {
            fetch()
        }
    }
}

suspend fun fetch() {}
