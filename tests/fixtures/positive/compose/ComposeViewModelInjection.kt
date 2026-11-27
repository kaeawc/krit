package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.lifecycle.viewmodel.compose.viewModel

@Composable
fun Home() {
    val vm = viewModel<HomeScreenViewModel>()
    Text("$vm")
}
