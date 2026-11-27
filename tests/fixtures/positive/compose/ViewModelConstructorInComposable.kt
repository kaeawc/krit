package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.lifecycle.ViewModel

class CounterViewModel : ViewModel()

@Composable
fun Counter() {
    val viewModel = CounterViewModel()
}
