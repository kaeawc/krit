package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

@Composable
fun Orientation(): Int = LocalContext.current.resources.configuration.orientation
