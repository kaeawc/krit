// Compiler-test source stubs; never packaged in the production artifact.
package androidx.fragment.app

import android.app.Dialog
import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.annotation.CallSuper
import androidx.annotation.MainThread
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.savedstate.SavedStateRegistry
import androidx.savedstate.SavedStateRegistryOwner

open class Fragment : LifecycleOwner, ViewModelStoreOwner, SavedStateRegistryOwner {
    constructor()

    constructor(contentLayoutId: Int)

    override val lifecycle: Lifecycle
        get() = TODO()

    override val viewModelStore: ViewModelStore
        get() = TODO()

    override val savedStateRegistry: SavedStateRegistry
        get() = TODO()

    @get:MainThread
    open val viewLifecycleOwner: LifecycleOwner
        get() = TODO()

    // Java @Nullable getters: null before attach / after detach.
    val context: Context?
        get() = TODO()

    val activity: FragmentActivity?
        get() = TODO()

    open val view: View?
        get() = TODO()

    var arguments: Bundle?
        get() = TODO()
        set(value) = TODO()

    val isAdded: Boolean
        get() = TODO()

    val parentFragmentManager: FragmentManager
        get() = TODO()

    val childFragmentManager: FragmentManager
        get() = TODO()

    fun requireContext(): Context = TODO()

    fun requireActivity(): FragmentActivity = TODO()

    fun requireArguments(): Bundle = TODO()

    fun requireView(): View = TODO()

    fun getString(resId: Int): String = TODO()

    fun getString(resId: Int, vararg formatArgs: Any?): String = TODO()

    open fun startActivity(intent: Intent) {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onAttach(context: Context) {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onCreate(savedInstanceState: Bundle?) {
        TODO()
    }

    @MainThread
    open fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View? = TODO()

    @MainThread
    open fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onStart() {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onResume() {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onPause() {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onStop() {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onDestroyView() {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onDestroy() {
        TODO()
    }

    @CallSuper
    @MainThread
    open fun onDetach() {
        TODO()
    }

    @MainThread
    open fun onSaveInstanceState(outState: Bundle) {
        TODO()
    }
}

open class FragmentActivity : androidx.activity.ComponentActivity {
    constructor()

    constructor(contentLayoutId: Int)

    open val supportFragmentManager: FragmentManager
        get() = TODO()
}

open class DialogFragment : Fragment() {
    open val dialog: Dialog?
        get() = TODO()

    open var isCancelable: Boolean
        get() = TODO()
        set(value) = TODO()

    @MainThread
    open fun onCreateDialog(savedInstanceState: Bundle?): Dialog = TODO()

    open fun show(manager: FragmentManager, tag: String?) {
        TODO()
    }

    open fun dismiss() {
        TODO()
    }

    fun requireDialog(): Dialog = TODO()
}

abstract class FragmentManager {
    abstract val fragments: List<Fragment>

    abstract fun beginTransaction(): FragmentTransaction

    abstract fun findFragmentByTag(tag: String?): Fragment?

    abstract fun findFragmentById(id: Int): Fragment?

    abstract fun popBackStack()

    abstract fun executePendingTransactions(): Boolean
}

abstract class FragmentTransaction {
    open fun add(containerViewId: Int, fragment: Fragment): FragmentTransaction = TODO()

    open fun add(containerViewId: Int, fragment: Fragment, tag: String?): FragmentTransaction = TODO()

    open fun replace(containerViewId: Int, fragment: Fragment): FragmentTransaction = TODO()

    open fun replace(containerViewId: Int, fragment: Fragment, tag: String?): FragmentTransaction = TODO()

    open fun remove(fragment: Fragment): FragmentTransaction = TODO()

    open fun addToBackStack(name: String?): FragmentTransaction = TODO()

    abstract fun commit(): Int

    abstract fun commitAllowingStateLoss(): Int

    abstract fun commitNow()
}

@MainThread
inline fun <reified VM : ViewModel> Fragment.viewModels(
    noinline ownerProducer: () -> ViewModelStoreOwner = { this },
    noinline factoryProducer: (() -> ViewModelProvider.Factory)? = null,
): Lazy<VM> = TODO()

@MainThread
inline fun <reified VM : ViewModel> Fragment.activityViewModels(
    noinline factoryProducer: (() -> ViewModelProvider.Factory)? = null,
): Lazy<VM> = TODO()
