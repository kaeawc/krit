package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Counter(): Int {
    Text("count")
    return 1
}
