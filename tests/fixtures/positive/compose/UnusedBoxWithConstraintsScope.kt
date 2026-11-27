package fixtures.compose

import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    BoxWithConstraints {
        Text("The constraints are never read")
    }
}
