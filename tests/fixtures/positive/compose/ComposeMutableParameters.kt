package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState

@Composable
fun Editor(lines: MutableList<String>, state: MutableState<Int>, cache: HashMap<String, Int>) {
    Text("${lines.size} ${state.value} ${cache.size}")
}
