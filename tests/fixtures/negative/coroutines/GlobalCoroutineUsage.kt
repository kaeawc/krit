package coroutines

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

class Example(private val scope: CoroutineScope) {
    fun startWork() {
        scope.launch {
            println("doing work")
        }
    }
}
