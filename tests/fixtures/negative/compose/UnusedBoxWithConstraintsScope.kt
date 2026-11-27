package fixtures.compose

import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    BoxWithConstraints {
        if (maxWidth.value > 600f) Text("Wide") else Text("Narrow")
    }
    BoxWithConstraints { Text("$maxHeight") }
}
