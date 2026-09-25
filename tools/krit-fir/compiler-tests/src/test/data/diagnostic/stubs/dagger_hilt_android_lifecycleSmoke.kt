// Smoke: @HiltViewModel on an @Inject-constructed ViewModel.
package stubs

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class SearchViewModel @Inject constructor(private val savedStateHandle: SavedStateHandle) : ViewModel() {
    val query: StateFlow<String> = savedStateHandle.getStateFlow("query", "")

    fun update(value: String) {
        savedStateHandle["query"] = value
        viewModelScope.launch {}
    }
}
