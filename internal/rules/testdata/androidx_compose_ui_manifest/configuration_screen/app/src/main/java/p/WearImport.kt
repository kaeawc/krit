package p.wear

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration
import androidx.wear.compose.material.Text

@Composable
fun WearScreen() {
    val w = LocalConfiguration.current.screenWidthDp
}
