package fixtures.compose

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card(title: String, modifier: Modifier = Modifier) {
    Box(modifier) {}
}

interface Screen {
    @Composable
    fun Content(modifier: Modifier)
}

class ScreenImpl : Screen {
    @Composable
    override fun Content(modifier: Modifier) {
        Box(modifier) {}
    }
}

@Composable
fun Modifier.Decorated(other: Modifier) {
    Box(this.then(other)) {}
}

fun notComposable(modifier: Modifier) = modifier
