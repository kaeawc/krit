// Smoke: viewModel() as a composable default argument and with a key.
package stubs

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.viewModel

class ComposeSmokeViewModel : ViewModel()

@Composable
fun VmScreen(viewModel: ComposeSmokeViewModel = viewModel()) {
    val keyed: ComposeSmokeViewModel = viewModel(key = "k")
    Text("$viewModel $keyed ${LocalViewModelStoreOwner.current}")
}
