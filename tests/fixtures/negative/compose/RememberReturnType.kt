package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember

class Analytics {
    fun track(event: String): Boolean = true
}

@Composable
fun Screen(analytics: Analytics) {
    val tracked = remember { analytics.track("shown") }
    val state = remember { mutableStateOf(0) }
    SideEffect { analytics.track("shown") }
}
