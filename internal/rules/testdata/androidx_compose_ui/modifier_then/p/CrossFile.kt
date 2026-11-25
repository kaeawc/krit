package p.then

import androidx.compose.ui.Modifier

fun Modifier.crossFile(): Modifier = then(otherFactory())
