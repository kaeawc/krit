package fixtures.positive.coroutines

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.MutableStateFlow

suspend fun getFlow(): Flow<Int> = flow {
    emit(1)
    emit(2)
}

suspend fun getState(): StateFlow<String> = MutableStateFlow("hello")
