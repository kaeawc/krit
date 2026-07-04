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

// A type with the same property name as kotlinx.coroutines.Dispatchers
// but a different fully-qualified name. The rule should rely on the
// receiver type (kotlinx.coroutines.Dispatchers) rather than just the
// property name, so this must NOT trigger the rule.
object LocalDispatchers {
    val IO: CoroutineDispatcher = TODO()
}

suspend fun localDispatcherLookalike() {
    withContext(LocalDispatchers.IO) { fetchFromNetwork() }
}

interface DispatcherProvider {
    val io: CoroutineDispatcher
}

// Top-level function: there is no enclosing class/object to inject a
// dispatcher into, so a hardcoded Dispatchers.* here is not actionable
// and must NOT be flagged.
fun loadTopLevel() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}

// Top-level extension function: the receiver is fixed by the call site
// and there is no enclosing type to inject into, so this must NOT be
// flagged either.
fun String.loadExt() {
    withContext(Dispatchers.Default) { fetchFromNetwork() }
}

// Generic top-level extension: same reasoning; the receiver-type AST
// shape carries type parameters but is still an extension.
suspend fun <T> T.loadGenericExt() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}

fun fetchFromNetwork() = Unit
fun renderResult() = Unit

open class ViewModel { val viewModelScope: CoroutineScope = TODO() }
