// Smoke: BasicText and BasicTextField.
package stubs

import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun TextSmoke() {
    var value by remember { mutableStateOf("") }
    BasicText("hello", modifier = Modifier.padding(4.dp), maxLines = 1)
    BasicTextField(value = value, onValueChange = { value = it }, singleLine = true)
}
