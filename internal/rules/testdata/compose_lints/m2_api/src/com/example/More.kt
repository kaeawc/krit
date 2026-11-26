package com.example.more

import androidx.compose.material.*
import androidx.compose.material.LocalContentColor
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color

@Composable
fun More(modifier: Modifier = Modifier) {
    val color = LocalContentColor.current
    val colors = Colors(
        Color.Red, Color.Red, Color.Red, Color.Red, Color.Red, Color.Red,
        Color.Red, Color.Red, Color.Red, Color.Red, Color.Red, Color.Red, true,
    )
    Divider(color = color)
    TextButton(onClick = {}) {}
    val typography = MaterialTheme.typography.body1
    val shape = MaterialTheme.shapes.small
    val elevation = ButtonDefaults.elevation()
    println(colors)
    println(typography)
    println(shape)
    println(elevation)
}

@Composable
fun Divider(color: Color) {
    println(color)
}
