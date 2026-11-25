package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

class Analytics {
    fun track(event: String) {}
}

@Composable
fun Screen(analytics: Analytics) {
    remember { analytics.track("shown") }
}
