package test

import kotlinx.coroutines.async
import kotlinx.coroutines.supervisorScope

suspend fun handle() = supervisorScope {
    val a = async { fetchA() }
    val b = async { fetchB() }
    a.await() to b.await()
}

suspend fun fetchA(): String = ""
suspend fun fetchB(): String = ""
