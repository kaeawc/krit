package fixtures.negative.coroutines

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

fun getFlow(): Flow<Int> = flow {
    emit(1)
    emit(2)
}
