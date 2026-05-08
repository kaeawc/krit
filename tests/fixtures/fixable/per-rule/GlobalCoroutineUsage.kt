package coroutines

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch

class Example {
    fun startWork() {
        GlobalScope.launch {
            println("doing work")
        }
    }
}
