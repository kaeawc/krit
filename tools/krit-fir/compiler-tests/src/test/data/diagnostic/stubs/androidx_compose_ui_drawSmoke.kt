// Smoke: clip with real Shapes and alpha.
package stubs

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.unit.dp

@Composable
fun DrawSmoke() {
    Box(Modifier.clip(RoundedCornerShape(4.dp)).clip(CircleShape).clip(RectangleShape).alpha(0.5f)) {}
}
