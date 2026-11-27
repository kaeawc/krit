package com.example

import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Labels() {
    BoxWithConstraints { Text("x", modifier = Modifier.defaultMinSize(minWidth = 48.dp, minHeight = 48.dp)) }
    BoxWithConstraints { Text("x", modifier = Modifier.defaultMinSize(48.dp, 48.dp)) }
}
