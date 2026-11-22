// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import javax.inject.Singleton

@Singleton
class ContextDispatcher(private val mainDispatcher: kotlin.coroutines.CoroutineContext) {
    private var value = 0
    fun bump() { value++ }
}

@Singleton
class LazyHandler(private val mainHandler: dagger.Lazy<android.os.Handler>) {
    private var value = 0
    fun bump() { value++ }
}

@Singleton
class ProviderHandler(private val uiHandler: javax.inject.Provider<android.os.Handler>) {
    private var value = 0
    fun bump() { value++ }
}

@Singleton
class ProviderDispatcher(private val uiDispatcher: javax.inject.Provider<kotlinx.coroutines.CoroutineDispatcher>) {
    private var value = 0
    fun bump() { value++ }
}

@Singleton
class LazyExecutor(private val mainThreadDispatcher: dagger.Lazy<java.util.concurrent.Executor>) {
    private var value = 0
    fun bump() { value++ }
}

@Singleton
class NameOnly(private val mainThreadHandler: String) {
    private var value = 0
    fun bump() { value++ }
}
