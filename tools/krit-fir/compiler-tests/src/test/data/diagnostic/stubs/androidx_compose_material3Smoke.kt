// Smoke: Material 3 Text/Icon/IconButton/TextField/Button call shapes.
package stubs

import androidx.compose.foundation.layout.Column
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.sp

@Composable
fun Material3Smoke(query: String, onQuery: (String) -> Unit, onClose: () -> Unit) {
    Column {
        Text("Title", color = Color.Red, fontSize = 18.sp, maxLines = 1)
        Text(text = "Body", modifier = Modifier.minimumInteractiveComponentSize())
        Icon(imageVector = Icons.Filled.Add, contentDescription = "Add", tint = Color.Black)
        IconButton(onClick = onClose, enabled = true) { Icon(Icons.Default.Close, contentDescription = "Close") }
        TextField(value = query, onValueChange = onQuery, label = { Text("Search") }, singleLine = true)
        OutlinedTextField(value = query, onValueChange = onQuery, placeholder = { Text("Type") }, readOnly = true)
        Button(onClick = onClose) { Text("OK") }
    }
}
