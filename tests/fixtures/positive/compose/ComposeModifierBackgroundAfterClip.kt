package fixtures.positive.compose

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

@Composable
fun ComposeModifierBackgroundAfterClipPositive() {
    Box(
        modifier = Modifier
            .size(48.dp)
            .background(Color.Red)
            .clip(RoundedCornerShape(8.dp)),
    )
}
