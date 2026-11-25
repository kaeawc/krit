package p

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.colorResource
import androidx.compose.ui.unit.dp

fun Modifier.noComposable(): Modifier = composed { padding(1.dp) }
fun Modifier.noComposableQualified(): Modifier = this.composed { Modifier.padding(1.dp).background(Color.Red) }
fun Modifier.withRemember(): Modifier = composed {
    val state = remember { 1 }
    padding(state.dp)
}
fun Modifier.withLocal(): Modifier = composed {
    val density = LocalDensity.current
    padding(density.density.dp)
}
fun Modifier.withResource(): Modifier = composed { background(colorResource(1)) }
fun Modifier.withSameFile(): Modifier = composed { myComposable() }
fun Modifier.withInspector(): Modifier = composed(inspectorInfo = {}) { padding(1.dp) }
fun Modifier.namedFactory(): Modifier = composed(factory = { padding(1.dp) })
fun Modifier.callsHelper(): Modifier = composed { helper() }
fun Modifier.empty(): Modifier = composed { this }
fun Modifier.nestedComposableLambda(): Modifier = composed {
    val f = @Composable { remember { 1 } }
    this
}

@Composable
fun Modifier.myComposable(): Modifier = this

fun Modifier.helper(): Modifier = padding(2.dp)
