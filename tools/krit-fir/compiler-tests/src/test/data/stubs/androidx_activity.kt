// Compiler-test source stubs; never packaged in the production artifact.
package androidx.activity

import android.os.Bundle
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.savedstate.SavedStateRegistry
import androidx.savedstate.SavedStateRegistryOwner

// Real chain: androidx.activity.ComponentActivity -> androidx.core.app.ComponentActivity
// -> android.app.Activity, implementing the lifecycle/view-model/saved-state owners.
open class ComponentActivity :
    androidx.core.app.ComponentActivity,
    LifecycleOwner,
    ViewModelStoreOwner,
    SavedStateRegistryOwner,
    OnBackPressedDispatcherOwner {
    constructor()

    constructor(contentLayoutId: Int)

    override val lifecycle: Lifecycle
        get() = TODO()

    override val viewModelStore: ViewModelStore
        get() = TODO()

    override val savedStateRegistry: SavedStateRegistry
        get() = TODO()

    override val onBackPressedDispatcher: OnBackPressedDispatcher
        get() = TODO()

    override fun onCreate(savedInstanceState: Bundle?) {
        TODO()
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        TODO()
    }
}

interface OnBackPressedDispatcherOwner : LifecycleOwner {
    val onBackPressedDispatcher: OnBackPressedDispatcher
}

class OnBackPressedDispatcher {
    fun addCallback(onBackPressedCallback: OnBackPressedCallback) {
        TODO()
    }

    fun addCallback(owner: LifecycleOwner, onBackPressedCallback: OnBackPressedCallback) {
        TODO()
    }

    fun onBackPressed() {
        TODO()
    }

    fun hasEnabledCallbacks(): Boolean = TODO()
}

abstract class OnBackPressedCallback(enabled: Boolean) {
    var isEnabled: Boolean
        get() = TODO()
        set(value) = TODO()

    abstract fun handleOnBackPressed()

    fun remove() {
        TODO()
    }
}

fun OnBackPressedDispatcher.addCallback(
    owner: LifecycleOwner? = null,
    enabled: Boolean = true,
    onBackPressed: OnBackPressedCallback.() -> Unit,
): OnBackPressedCallback = TODO()

inline fun <reified VM : ViewModel> ComponentActivity.viewModels(
    noinline factoryProducer: (() -> ViewModelProvider.Factory)? = null,
): Lazy<VM> = TODO()
