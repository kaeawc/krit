package fixtures.compose

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.Modifier as M

@Composable
fun Card(title: String, modifier: Modifier) {
    Box(modifier) {}
}

@Composable
fun Optional(
    modifier: Modifier?,
    enabled: Boolean = true,
) {
    Box(modifier ?: Modifier) {}
}

// The default is written in the parameter type's own spelling.
@Composable
fun GlanceWidget(text: String, modifier: androidx.glance.GlanceModifier) {
}

@Composable
fun Qualified(modifier: androidx.compose.ui.Modifier) {
    Box(modifier) {}
}

@Composable
fun Aliased(m: M) {
    Box(m) {}
}

// No fix: a vararg parameter's default would have to be an array.
@Composable
fun Many(vararg modifier: Modifier) {
    Box(Modifier) {}
}

// No fix: this GlanceModifier is a lookalike, so its companion is no Modifier.
class GlanceModifier

@Composable
fun Lookalike(modifier: GlanceModifier) {
    Box(Modifier) {}
}
