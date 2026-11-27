package fixtures.compose

import androidx.compose.material.Button
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.viewModel

class CounterViewModel : ViewModel()
class CounterState

@Composable
fun Counter() {
    val viewModel: CounterViewModel = viewModel()
    val state = CounterState()
    Button(onClick = { CounterViewModel() }) { Text("not in composition") }
}

fun factory() = CounterViewModel()
