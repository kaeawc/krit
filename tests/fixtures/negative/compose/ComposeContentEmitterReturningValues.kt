package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Counter() {
    Text("count")
}

@Composable
fun rememberCount(): Int {
    return remember { 1 }
}
