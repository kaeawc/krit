package fixtures.compose

import androidx.compose.foundation.layout.RowScope
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable

@Stable
class StablePresenter {
    @Composable
    fun Render() {}
}

enum class Tone {
    Loud;

    @Composable
    fun Render() {}
}

@Composable
fun RowScope.Item() {}

@Composable
fun String.Label() {}

object Screens {
    @Composable
    fun Home() {}
}
