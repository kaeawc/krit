package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

suspend fun directWithContext() {
    withContext(Dispatchers.IO) {
        fetchFromNetwork()
    }
}

suspend fun directLaunch() {
    launch(Dispatchers.Default) {
        fetchFromNetwork()
    }
}

suspend fun withMainDispatcher() {
    withContext(Dispatchers.Main) {
        updateUI()
    }
}

fun flowWithHardcodedDispatcher() = flow { emit(fetchFromNetwork()) }.flowOn(Dispatchers.IO)

fun createScopeHardcoded() {
    CoroutineScope(Dispatchers.IO).launch { fetchFromNetwork() }
}

class MyViewModel : ViewModel() {
    fun loadFromViewModelScope() {
        viewModelScope.launch(Dispatchers.IO) { fetchFromNetwork() }
    }

    fun asyncFromViewModelScope() {
        viewModelScope.async(Dispatchers.Default) { fetchFromNetwork() }
    }
}

class MyFragment : Fragment() {
    fun loadFromLifecycleScope() {
        lifecycleScope.launch(Dispatchers.IO) { fetchFromNetwork() }
    }

    fun launchWhenStarted() {
        lifecycleScope.launchWhenStarted { withContext(Dispatchers.IO) { fetchFromNetwork() } }
    }
}

object UtilObject {
    suspend fun doWork() {
        withContext(Dispatchers.IO) { fetchFromNetwork() }
    }
}

class MyClass {
    companion object {
        suspend fun doWork() {
            withContext(Dispatchers.IO) { fetchFromNetwork() }
        }
    }
}

class JavaInterop {
    companion object {
        @JvmStatic
        fun doWorkStatic() {
            CoroutineScope(Dispatchers.IO).launch { fetchFromNetwork() }
        }
    }
}

fun fetchFromNetwork() = Unit
fun updateUI() = Unit

open class ViewModel { val viewModelScope = CoroutineScope(Dispatchers.Main) }
open class Fragment { val lifecycleScope = CoroutineScope(Dispatchers.Main) }
