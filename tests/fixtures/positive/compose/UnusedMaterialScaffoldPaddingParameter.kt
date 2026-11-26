package fixtures.compose

import androidx.compose.material.Scaffold
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Screen(names: List<String>) {
    Scaffold { Text("Hello") }
    Scaffold { innerPadding -> Text("Hello") }
    Scaffold { names.forEach { Text(it) } }
}
