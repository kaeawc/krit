package fixtures.compose

import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Screen(names: List<String>) {
    Scaffold { Text("Hello") }
    Scaffold { innerPadding -> Text("Hello") }
    Scaffold { names.forEach { Text(it) } }
}
