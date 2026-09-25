// Smoke: Material (M2) Text/Icon/IconButton/TextField/Button call shapes.
package stubs

import androidx.compose.foundation.layout.Column
import androidx.compose.material.Button
import androidx.compose.material.Icon
import androidx.compose.material.IconButton
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.material.TextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.sp

@Composable
fun MaterialSmoke(query: String, onQuery: (String) -> Unit, onClose: () -> Unit) {
    Column {
        Text("Title", color = Color.Red, fontSize = 18.sp, maxLines = 1)
        Icon(Icons.Default.Close, contentDescription = "Close", tint = Color.Black)
        IconButton(onClick = onClose) { Icon(Icons.Filled.Menu, contentDescription = null) }
        TextField(value = query, onValueChange = onQuery, label = { Text("Search") }, singleLine = true)
        OutlinedTextField(value = query, onValueChange = onQuery, placeholder = { Text("Type") })
        Button(onClick = onClose, modifier = Modifier.minimumInteractiveComponentSize()) { Text("OK") }
        Icon(painter = painterResource(android.R.drawable.ic_menu_add), contentDescription = "Add")
    }
}
