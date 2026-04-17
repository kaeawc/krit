package coroutines

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun fetchData(ioDispatcher: CoroutineDispatcher = Dispatchers.IO): String {
    return withContext(ioDispatcher) {
        "data"
    }
}
