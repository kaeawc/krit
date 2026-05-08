package coroutines

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

fun CoroutineScope.doWork() {
    launch {
        println("working")
    }
}
