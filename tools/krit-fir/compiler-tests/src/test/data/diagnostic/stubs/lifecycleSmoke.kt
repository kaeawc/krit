// Smoke: lifecycleScope + repeatOnLifecycle, observers, ViewModel scope, LiveData.
package stubs

import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData
import androidx.lifecycle.Observer
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.flowWithLifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.launch

class LifecycleSmokeViewModel : ViewModel() {
    private val _name = MutableLiveData<String>()
    val name: LiveData<String> = _name

    fun load() {
        viewModelScope.launch { _name.value = "loaded" }
        _name.postValue("posting")
    }

    override fun onCleared() {
        super.onCleared()
    }
}

class LoggingObserver : DefaultLifecycleObserver {
    override fun onStart(owner: LifecycleOwner) {
        <!PrintlnInProduction!>println<!>(owner.lifecycle.currentState)
    }
}

fun observe(owner: LifecycleOwner, flow: Flow<Int>, viewModel: LifecycleSmokeViewModel) {
    owner.lifecycle.addObserver(LoggingObserver())
    owner.lifecycle.addObserver(LifecycleEventObserver { source, event ->
        if (event == Lifecycle.Event.ON_DESTROY) source.lifecycle.removeObserver(LoggingObserver())
    })
    owner.lifecycleScope.launch {
        owner.repeatOnLifecycle(Lifecycle.State.STARTED) {
            flow.collect { <!PrintlnInProduction!>println<!>(it) }
        }
    }
    owner.lifecycleScope.launch {
        owner.lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED) {}
        flow.flowWithLifecycle(owner.lifecycle, Lifecycle.State.STARTED).collect {}
    }
    viewModel.name.observe(owner, Observer { value -> <!PrintlnInProduction!>println<!>(value) })
    viewModel.name.observe(owner) { value -> <!PrintlnInProduction!>println<!>(value) }
    if (owner.lifecycle.currentState.isAtLeast(Lifecycle.State.CREATED)) viewModel.load()
}

fun provide(owner: ViewModelStoreOwner): LifecycleSmokeViewModel =
    ViewModelProvider(owner)[LifecycleSmokeViewModel::class.java]
