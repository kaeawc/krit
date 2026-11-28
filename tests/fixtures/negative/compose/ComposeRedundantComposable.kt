package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import coil.compose.AsyncImage

@Composable
fun Greeting(name: String) {
    Text("Hello, $name")
}

@Composable
fun rememberCount(): Int = remember { 1 }

@Composable
fun Remote(url: String) {
    AsyncImage(model = url, contentDescription = null)
}
