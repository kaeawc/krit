// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A function-typed or State parameter read only inside derivedStateOf is
// tracked by that block, so the keyless remember does not go stale.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.remember

fun use(x: Any?) {}

@Composable
fun Tracked(onClick: () -> Int, count: State<Int>) {
    val a = remember { derivedStateOf { onClick() } }
    val b = remember { derivedStateOf { count.value + 1 } }
    use(a)
    use(b)
}
