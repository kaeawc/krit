package fixtures.compose

import androidx.compose.animation.Crossfade
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Screen(loading: Boolean) {
    Crossfade(loading) { Text("Loading") }
    Crossfade(targetState = loading) { state -> Text("Done") }
}
