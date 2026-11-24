package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.rememberCoroutineScope
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

@Composable
fun Refresh(onClick: (() -> Unit) -> Unit) {
    val scope = rememberCoroutineScope()
    LaunchedEffect(Unit) { scope.launch { reload() } }
    onClick { scope.launch { reload() } }
    val handler = { scope.launch { reload() } }
}

class Launcher {
    fun launch(input: String) {}
}

@Composable
fun PickImage(launcher: Launcher) {
    // A member named launch is not kotlinx.coroutines.launch.
    launcher.launch("image/*")
}

fun outsideComposition(scope: CoroutineScope) {
    scope.launch { reload() }
}

suspend fun reload() {}
