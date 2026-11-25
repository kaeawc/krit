package fixtures.compose

import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.dp

fun Modifier.remembered(): Modifier = composed {
    val size = remember { 8 }
    padding(size.dp)
}

fun Modifier.densityAware(): Modifier = composed {
    padding(LocalDensity.current.density.dp)
}

fun Modifier.fromOtherFile(): Modifier = composed { themedPadding() }
