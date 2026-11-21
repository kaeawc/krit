package app.cash.turbine

import kotlinx.coroutines.flow.Flow
import kotlin.time.Duration

interface ReceiveTurbine<T> {
    suspend fun awaitItem(): T
    suspend fun awaitComplete()
    suspend fun awaitError(): Throwable
    fun expectNoEvents()
    fun expectMostRecentItem(): T
    suspend fun cancelAndIgnoreRemainingEvents()
}

interface TurbineTestContext<T> : ReceiveTurbine<T>

suspend fun <T> Flow<T>.test(
    timeout: Duration? = null,
    name: String? = null,
    validate: suspend TurbineTestContext<T>.() -> Unit,
) {
    TODO()
}
