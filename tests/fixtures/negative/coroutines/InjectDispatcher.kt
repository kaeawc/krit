package test

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.launch
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

class MyViewModel(
    private val ioDispatcher: CoroutineDispatcher,
    private val mainDispatcher: CoroutineDispatcher,
) : ViewModel() {
    fun loadData() {
        viewModelScope.launch(ioDispatcher) { fetchFromNetwork() }
    }

    fun updateUI() {
        viewModelScope.launch(mainDispatcher) { renderResult() }
    }
}

object UtilObject {
    suspend fun doWork(dispatcher: CoroutineDispatcher = Dispatchers.IO) {
        withContext(dispatcher) { fetchFromNetwork() }
    }

    fun getFlow(dispatcher: CoroutineDispatcher = Dispatchers.IO) =
        flow { emit(fetchFromNetwork()) }.flowOn(dispatcher)
}

class MyClass {
    companion object {
        suspend fun doWork(dispatcher: CoroutineDispatcher = Dispatchers.IO) {
            withContext(dispatcher) { fetchFromNetwork() }
        }
    }
}

object LocalDispatchers {
    val IO: String = "io"
}

fun localDispatcherLookalike() {
    withContext(LocalDispatchers.IO) { fetchFromNetwork() }
}

interface DispatcherProvider {
    val io: CoroutineDispatcher
}

fun fetchFromNetwork() = Unit
fun renderResult() = Unit

open class ViewModel { val viewModelScope: CoroutineScope = TODO() }
