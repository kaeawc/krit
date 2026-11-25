package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

fun refresh() {}

@Composable
fun Screen() {
    LaunchedEffect(refresh()) { }
}
