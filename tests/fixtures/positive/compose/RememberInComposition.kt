package fixtures.compose

import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.runtime.Composable

@Composable
fun PressableCard(onSource: (MutableInteractionSource) -> Unit) {
    val source = MutableInteractionSource()
    onSource(source)
}
