package u

import androidx.compose.runtime.Composable

class Context
class View

fun <T> retain(block: () -> T): T = block()

object LocalContext { val current = Context() }

@Composable
fun Lookalikes(context: Context) {
    retain { context }
    retain { View() }
    retain { LocalContext.current }
    retain { Unit }
}
