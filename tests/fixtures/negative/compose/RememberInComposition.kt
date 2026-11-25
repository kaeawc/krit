package fixtures.compose

import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun PressableCard(onSource: (MutableInteractionSource) -> Unit) {
    val source = remember { MutableInteractionSource() }
    onSource(source)
}

class Sources {
    val shared = MutableInteractionSource()
}
