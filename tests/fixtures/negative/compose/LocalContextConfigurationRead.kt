package fixtures.compose

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration

@Composable
fun Orientation(): Int = LocalConfiguration.current.orientation

fun orientationOf(context: Context): Int = context.resources.configuration.orientation
