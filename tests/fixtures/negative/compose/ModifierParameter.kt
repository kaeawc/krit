package fixtures.compose

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card(title: String, modifier: Modifier = Modifier, enabled: Boolean = true) {
    Box(modifier)
}

@Composable
fun cardModifier(mod: Modifier): Modifier = mod

fun NotComposable(mod: Modifier) {
}

@Composable
fun Outer() {
    @Composable
    fun Local(mod: Modifier) {
        Box(mod)
    }
}
