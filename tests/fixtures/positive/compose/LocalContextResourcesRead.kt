package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

@Composable
fun Density(): Float = LocalContext.current.resources.displayMetrics.density
