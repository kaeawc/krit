package w

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration

@Composable
fun WatchScreen() {
    val w = LocalConfiguration.current.screenWidthDp
}
