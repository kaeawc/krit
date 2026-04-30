package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Greeting(name: String) {
    // Uses a positional placeholder, no concatenation.
    Text(stringResource(R.string.greeting_with_name, name))
}

@Composable
fun StaticLabel() {
    // Concatenation with literals only is fine; nothing dynamic to reorder.
    Text(stringResource(R.string.greeting) + "!")
}
