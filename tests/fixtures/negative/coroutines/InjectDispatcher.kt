package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Repository(
    private val ioDispatcher: CoroutineDispatcher,
    provider: DispatcherProvider,
) {
    private val defaultDispatcher: CoroutineDispatcher = ioDispatcher
    private val providerDispatcher: CoroutineDispatcher = provider.io

    suspend fun fromFunctionParam(dispatcher: CoroutineDispatcher = Dispatchers.IO) {
        withContext(dispatcher) { fetchFromNetwork() }
    }

    suspend fun fromConstructorParam() {
        withContext(ioDispatcher) { fetchFromNetwork() }
    }

    suspend fun fromClassProperty() {
        withContext(defaultDispatcher) { fetchFromNetwork() }
    }

    suspend fun fromProvider() {
        withContext(providerDispatcher) { fetchFromNetwork() }
    }
}

interface DispatcherProvider {
    val io: CoroutineDispatcher
}

fun fetchFromNetwork() = Unit
