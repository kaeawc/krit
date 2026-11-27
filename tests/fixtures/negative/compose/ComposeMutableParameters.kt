package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.State

@Composable
fun Viewer(lines: List<String>, state: State<Int>, label: String) {
    Text("${lines.size} ${state.value} $label")
}

fun notComposable(lines: MutableList<String>) = lines.size
