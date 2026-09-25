// Compiler-test source stubs; never packaged in the production artifact.
package androidx.lifecycle

import kotlin.coroutines.CoroutineContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow

abstract class Lifecycle {
    abstract val currentState: State

    open val currentStateFlow: StateFlow<State>
        get() = TODO()

    abstract fun addObserver(observer: LifecycleObserver)

    abstract fun removeObserver(observer: LifecycleObserver)

    enum class Event {
        ON_CREATE,
        ON_START,
        ON_RESUME,
        ON_PAUSE,
        ON_STOP,
        ON_DESTROY,
        ON_ANY,
        ;

        val targetState: State
            get() = TODO()
    }

    enum class State {
        DESTROYED,
        INITIALIZED,
        CREATED,
        STARTED,
        RESUMED,
        ;

        fun isAtLeast(state: State): Boolean = TODO()
    }
}

interface LifecycleOwner {
    val lifecycle: Lifecycle
}

interface LifecycleObserver

fun interface LifecycleEventObserver : LifecycleObserver {
    fun onStateChanged(source: LifecycleOwner, event: Lifecycle.Event)
}

interface DefaultLifecycleObserver : LifecycleObserver {
    fun onCreate(owner: LifecycleOwner) {}

    fun onStart(owner: LifecycleOwner) {}

    fun onResume(owner: LifecycleOwner) {}

    fun onPause(owner: LifecycleOwner) {}

    fun onStop(owner: LifecycleOwner) {}

    fun onDestroy(owner: LifecycleOwner) {}
}

open class LifecycleRegistry(provider: LifecycleOwner) : Lifecycle() {
    override var currentState: State
        get() = TODO()
        set(value) = TODO()

    override fun addObserver(observer: LifecycleObserver) {
        TODO()
    }

    override fun removeObserver(observer: LifecycleObserver) {
        TODO()
    }

    open fun handleLifecycleEvent(event: Event) {
        TODO()
    }
}

// lifecycle-runtime-ktx: the owner-bound coroutine scope. The deprecated
// launchWhenX builders live here (not in kotlinx.coroutines); they only pause
// the block, they do not cancel it.
abstract class LifecycleCoroutineScope : CoroutineScope {
    @Deprecated("launchWhenCreated is deprecated as it can lead to wasted resources in some cases.")
    fun launchWhenCreated(block: suspend CoroutineScope.() -> Unit): Job = TODO()

    @Deprecated("launchWhenStarted is deprecated as it can lead to wasted resources in some cases.")
    fun launchWhenStarted(block: suspend CoroutineScope.() -> Unit): Job = TODO()

    @Deprecated("launchWhenResumed is deprecated as it can lead to wasted resources in some cases.")
    fun launchWhenResumed(block: suspend CoroutineScope.() -> Unit): Job = TODO()
}

val LifecycleOwner.lifecycleScope: LifecycleCoroutineScope
    get() = TODO()

val Lifecycle.coroutineScope: LifecycleCoroutineScope
    get() = TODO()

// The real API is a suspend extension on LifecycleOwner / Lifecycle; there is
// no receiver-less, non-suspend repeatOnLifecycle.
suspend fun LifecycleOwner.repeatOnLifecycle(state: Lifecycle.State, block: suspend CoroutineScope.() -> Unit) {
    TODO()
}

suspend fun Lifecycle.repeatOnLifecycle(state: Lifecycle.State, block: suspend CoroutineScope.() -> Unit) {
    TODO()
}

fun <T> Flow<T>.flowWithLifecycle(
    lifecycle: Lifecycle,
    minActiveState: Lifecycle.State = Lifecycle.State.STARTED,
): Flow<T> = TODO()

abstract class ViewModel {
    constructor()

    constructor(vararg closeables: AutoCloseable)

    protected open fun onCleared() {
        TODO()
    }

    fun addCloseable(closeable: AutoCloseable) {
        TODO()
    }
}

val ViewModel.viewModelScope: CoroutineScope
    get() = TODO()

open class ViewModelStore {
    fun clear() {
        TODO()
    }
}

interface ViewModelStoreOwner {
    val viewModelStore: ViewModelStore
}

open class ViewModelProvider(owner: ViewModelStoreOwner) {
    constructor(owner: ViewModelStoreOwner, factory: Factory) : this(owner)

    open operator fun <T : ViewModel> get(modelClass: Class<T>): T = TODO()

    open operator fun <T : ViewModel> get(key: String, modelClass: Class<T>): T = TODO()

    interface Factory {
        fun <T : ViewModel> create(modelClass: Class<T>): T = TODO()
    }
}

class SavedStateHandle {
    constructor()

    constructor(initialState: Map<String, Any?>)

    operator fun contains(key: String): Boolean = TODO()

    operator fun <T> get(key: String): T? = TODO()

    operator fun <T> set(key: String, value: T?) {
        TODO()
    }

    fun <T> getStateFlow(key: String, initialValue: T): StateFlow<T> = TODO()

    fun <T> getLiveData(key: String): MutableLiveData<T> = TODO()

    fun <T> remove(key: String): T? = TODO()

    fun keys(): Set<String> = TODO()
}

abstract class LiveData<T> {
    constructor()

    constructor(value: T)

    // Java getValue() is @Nullable; setValue/postValue are protected here and
    // made public by MutableLiveData.
    open val value: T?
        get() = TODO()

    open fun observe(owner: LifecycleOwner, observer: Observer<in T>) {
        TODO()
    }

    open fun observeForever(observer: Observer<in T>) {
        TODO()
    }

    open fun removeObserver(observer: Observer<in T>) {
        TODO()
    }

    open fun hasObservers(): Boolean = TODO()

    protected open fun onActive() {
        TODO()
    }

    protected open fun onInactive() {
        TODO()
    }
}

open class MutableLiveData<T> : LiveData<T> {
    constructor()

    constructor(value: T)

    override var value: T?
        get() = TODO()
        set(value) = TODO()

    open fun postValue(value: T) {
        TODO()
    }
}

fun interface Observer<T> {
    fun onChanged(value: T)
}

fun <T> Flow<T>.asLiveData(context: CoroutineContext = kotlin.coroutines.EmptyCoroutineContext): LiveData<T> = TODO()
