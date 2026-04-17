package test

import kotlinx.coroutines.supervisorScope

suspend fun handle() = supervisorScope {
    fetch()
}

suspend fun fetch() {}
