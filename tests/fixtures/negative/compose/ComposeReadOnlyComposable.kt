package fixtures.compose

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color

@Composable
@ReadOnlyComposable
fun primaryColor(): Color = MaterialTheme.colorScheme.primary

@Composable
fun rememberedColor(): Color {
    val color = MaterialTheme.colorScheme.primary
    return remember(color) { color }
}
