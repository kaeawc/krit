package fixtures.compose

import androidx.compose.runtime.Composable

@Composable
fun Card(title: String, body: @Composable () -> Unit) {}
