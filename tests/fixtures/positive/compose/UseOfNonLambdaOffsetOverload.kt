package fixtures.compose

import androidx.compose.animation.core.animateDpAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.offset
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Slide(expanded: Boolean) {
    val x by animateDpAsState(if (expanded) 100.dp else 0.dp)
    Box(Modifier.offset(x, 0.dp))
}
