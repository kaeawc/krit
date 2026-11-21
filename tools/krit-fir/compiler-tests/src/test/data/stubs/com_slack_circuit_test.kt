package com.slack.circuit.test

interface CircuitReceiveTurbine<T> : app.cash.turbine.ReceiveTurbine<T> {
    override suspend fun awaitItem(): T
}
class Presenter {
    suspend fun test(block: suspend CircuitReceiveTurbine<Int>.() -> Unit) {
        block(object : CircuitReceiveTurbine<Int> {
            override suspend fun awaitItem(): Int = 0
            override suspend fun awaitComplete() = Unit
            override suspend fun awaitError(): Throwable = TODO()
            override fun expectNoEvents() = Unit
            override fun expectMostRecentItem(): Int = 0
            override suspend fun cancelAndIgnoreRemainingEvents() = Unit
        })
    }
}
