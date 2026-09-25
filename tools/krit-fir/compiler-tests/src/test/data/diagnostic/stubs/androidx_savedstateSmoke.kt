// Smoke: register a saved-state provider on a SavedStateRegistryOwner.
package stubs

import android.os.Bundle
import androidx.lifecycle.LifecycleOwner
import androidx.savedstate.SavedStateRegistry
import androidx.savedstate.SavedStateRegistryOwner

fun registerState(owner: SavedStateRegistryOwner) {
    val registry: SavedStateRegistry = owner.savedStateRegistry
    registry.registerSavedStateProvider("smoke") { Bundle() }
    val restored: Bundle? = registry.consumeRestoredStateForKey("smoke")
    val lifecycleOwner: LifecycleOwner = owner
    <!PrintlnInProduction!>println<!>("$restored $lifecycleOwner")
}
