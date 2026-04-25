package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

suspend fun directWithContext() {
    withContext(Dispatchers.IO) {
        fetchFromNetwork()
    }
}

suspend fun directLaunch() {
    launch(Dispatchers.Default) {
        fetchFromNetwork()
    }
}

fun fetchFromNetwork() = Unit
