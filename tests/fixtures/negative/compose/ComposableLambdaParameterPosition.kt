package fixtures.compose

import androidx.compose.runtime.Composable

@Composable
fun Card(title: String, content: @Composable () -> Unit) {}

@Composable
fun Row(start: @Composable () -> Unit, end: @Composable () -> Unit, gap: Int) {}

@Composable
fun Click(onClick: () -> Unit, label: String) {}
