package fixtures.compose

import androidx.compose.runtime.Composable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

class CounterModel {
    private val _count = MutableStateFlow(0)
    val count: StateFlow<Int> = _count
}

@Composable
fun Counter(model: CounterModel) {
    val current = model.count.value
}
