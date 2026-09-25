// Smoke: ComponentActivity supertypes, back handling, and by viewModels().
package stubs

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.addCallback
import androidx.activity.viewModels
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelStoreOwner
import androidx.savedstate.SavedStateRegistryOwner

class SmokeActivityViewModel : ViewModel()

class SmokeComponentActivity : ComponentActivity() {
    private val viewModel: SmokeActivityViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        onBackPressedDispatcher.addCallback(
            this,
            object : OnBackPressedCallback(true) {
                override fun handleOnBackPressed() {
                    isEnabled = false
                }
            },
        )
        onBackPressedDispatcher.addCallback(this) { remove() }
        val owner: LifecycleOwner = this
        val store: ViewModelStoreOwner = this
        val saved: SavedStateRegistryOwner = this
        val core: androidx.core.app.ComponentActivity = this
        val platform: android.app.Activity = this
        <!PrintlnInProduction!>println<!>("$viewModel $owner $store $saved $core $platform ${lifecycle.currentState} ${viewModelStore}")
    }
}
