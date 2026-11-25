package fixtures.compose

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalResources

@Composable
fun Density(): Float = LocalResources.current.displayMetrics.density

fun densityOf(context: Context): Float = context.resources.displayMetrics.density
