package c

import androidx.compose.runtime.Composable
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

class Screen(val state: StateFlow<Int>, private val scope: CoroutineScope) {
    @Composable
    fun Content() {
        state.value
        scope.launch {}
    }
}
