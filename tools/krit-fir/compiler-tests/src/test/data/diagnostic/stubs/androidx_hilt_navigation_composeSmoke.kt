// Smoke: a @HiltViewModel injected into a composable via hiltViewModel().
package stubs

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject

@HiltViewModel
class HiltSmokeViewModel @Inject constructor(private val handle: SavedStateHandle) : ViewModel() {
    val id: String? = handle["id"]
}

@Composable
fun HiltScreen(viewModel: HiltSmokeViewModel = hiltViewModel()) {
    val keyed: HiltSmokeViewModel = hiltViewModel(key = "k")
    Text("${viewModel.id} $keyed")
}
