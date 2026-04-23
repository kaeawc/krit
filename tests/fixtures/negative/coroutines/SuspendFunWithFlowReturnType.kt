package fixtures.negative.coroutines

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.StateFlow

fun getFlow(): Flow<Int> = flow {
    emit(1)
    emit(2)
}

suspend fun suspendOnly(): Int = 42

// suspend fun example(): Flow<Int> = flow { emit(1) }
