package fixtures.positive.coroutines

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

suspend fun getFlow(): Flow<Int> = flow {
    emit(1)
    emit(2)
}
