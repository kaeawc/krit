package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.rememberCoroutineScope
import kotlinx.coroutines.launch

@Composable
fun Refresh() {
    val scope = rememberCoroutineScope()
    scope.launch { reload() }
}

suspend fun reload() {}
