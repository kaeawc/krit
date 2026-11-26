package fixtures.compose

import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionScope
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun Shared() {
    SharedTransitionScope { Box {} }
    SharedTransitionScope { modifier -> Box {} }
}
