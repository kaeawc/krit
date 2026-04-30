package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Greeting(value: String) {
    // Uses a positional placeholder, no template embedding.
    Text(stringResource(R.string.label_fmt, value))
}

@Composable
fun StaticLabel() {
    // Template with only static text around stringResource is fine; nothing dynamic to reorder.
    Text("${stringResource(R.string.label)}!")
}

@Composable
fun PlainTemplate(value: String) {
    // No stringResource interpolation; not this rule's concern.
    Text("Label: $value")
}
