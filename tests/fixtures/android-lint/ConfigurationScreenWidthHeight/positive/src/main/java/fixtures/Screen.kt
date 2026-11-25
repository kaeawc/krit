package fixtures

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration

@Composable
fun screenWidth(): Int = LocalConfiguration.current.screenWidthDp
