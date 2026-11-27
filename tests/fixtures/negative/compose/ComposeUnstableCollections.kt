package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import kotlinx.collections.immutable.ImmutableList

@Composable
fun Tags(tags: ImmutableList<String>, names: Array<String>, label: String) {
    Text("${tags.size} ${names.size} $label")
}

fun notComposable(tags: List<String>) = tags.size
