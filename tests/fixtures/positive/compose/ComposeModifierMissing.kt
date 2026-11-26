package fixtures.compose

import androidx.compose.foundation.layout.Column
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header(title: String) {
    Column {
        Text(title)
    }
}
