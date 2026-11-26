package fixtures.compose

import androidx.compose.animation.Crossfade
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Screen(loading: Boolean) {
    Crossfade(loading) { if (it) Text("Loading") else Text("Done") }
    Crossfade(targetState = loading) { state -> Text(if (state) "Loading" else "Done") }
}

class Local {
    @Composable
    fun Crossfade(value: Boolean, content: @Composable (Boolean) -> Unit) = content(value)

    @Composable
    fun Use() {
        Crossfade(true) { Text("a member Crossfade is not the animation API") }
    }
}
