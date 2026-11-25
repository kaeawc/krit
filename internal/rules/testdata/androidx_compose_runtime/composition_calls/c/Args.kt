package c

import androidx.compose.runtime.Composable
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

@Composable
fun WithArgs(scope: CoroutineScope) {
    scope.launch(Dispatchers.IO) {}
    scope.launch(start = CoroutineStart.LAZY) {}
    val s = flowOf(1).flowOn(Dispatchers.IO).stateIn(scope, SharingStarted.Eagerly, 0)
    s.value
    // scope.launch {} in a comment
    val text = "scope.launch {}"
}
