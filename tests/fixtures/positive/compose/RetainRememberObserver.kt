package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.RememberObserver
import androidx.compose.runtime.retain.retain

class Tracker : RememberObserver {
    override fun onRemembered() {}
    override fun onForgotten() {}
    override fun onAbandoned() {}
}

@Composable
fun Screen() {
    val tracker = retain { Tracker() }
}
