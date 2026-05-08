package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Greeting(name: String) {
    Text(stringResource(R.string.greeting) + " " + name)
}
