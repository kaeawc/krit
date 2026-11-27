package fixtures.compose

import androidx.compose.foundation.layout.Column
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Column {
        Text("title")
        Text("subtitle")
    }
}

@Composable
fun Choice(flag: Boolean) {
    if (flag) Text("a") else Text("b")
}
