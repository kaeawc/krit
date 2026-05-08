package coroutines

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

suspend fun CoroutineScope.doWork() {
    launch {
        println("working")
    }
}
