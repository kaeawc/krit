package fixtures.compose

import androidx.compose.runtime.Composable

@Composable
fun Card(title: String, content: @Composable () -> Unit) {}

@Composable
fun Row(start: @Composable () -> Unit, end: @Composable () -> Unit) {}

@Composable
fun List(itemContent: @Composable (Int) -> Unit) {}

@Composable
fun Click(onClick: () -> Unit) {}

@Composable
fun value(body: @Composable () -> Unit): Int = 1
