// Compiler-test source stubs; never packaged in the production artifact.
package androidx.savedstate

import android.os.Bundle
import androidx.lifecycle.LifecycleOwner

interface SavedStateRegistryOwner : LifecycleOwner {
    val savedStateRegistry: SavedStateRegistry
}

class SavedStateRegistry {
    val isRestored: Boolean
        get() = TODO()

    fun consumeRestoredStateForKey(key: String): Bundle? = TODO()

    fun registerSavedStateProvider(key: String, provider: SavedStateProvider) {
        TODO()
    }

    fun unregisterSavedStateProvider(key: String) {
        TODO()
    }

    fun interface SavedStateProvider {
        fun saveState(): Bundle
    }
}
