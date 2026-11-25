package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.RememberObserver
import androidx.compose.runtime.remember
import androidx.compose.runtime.retain.retain

class Tracker : RememberObserver {
    override fun onRemembered() {}
    override fun onForgotten() {}
    override fun onAbandoned() {}
}

class Plain

@Composable
fun Screen() {
    val tracker = remember { Tracker() }
    val plain = retain { Plain() }
}
