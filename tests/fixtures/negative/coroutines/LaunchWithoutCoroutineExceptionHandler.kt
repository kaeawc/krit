package test

import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch

fun start() {
    val handler = CoroutineExceptionHandler { _, throwable ->
        println(throwable)
    }
    GlobalScope.launch(handler) {
        throw RuntimeException("boom")
    }
}
