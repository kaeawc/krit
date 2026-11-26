package com.example

import androidx.compose.material.ButtonDefaults
import androidx.compose.material.MaterialTheme
import androidx.compose.material.Text
import androidx.compose.material.Surface as M2Surface
import androidx.compose.material.icons.Icons
import androidx.compose.material3.Button
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Screen(modifier: Modifier = Modifier) {
    Text("hello")
    M2Surface {
        Text(text = "inside", color = MaterialTheme.colors.primary)
    }
    val colors = ButtonDefaults.buttonColors()
    val shapes = MaterialTheme.shapes
    val alpha = MaterialTheme.colors.primary.copy(alpha = 0.5f)
    androidx.compose.material.Divider()
    Button(onClick = {}) {
        androidx.compose.material3.Text("m3")
    }
    val icons = Icons.Default
    println(colors)
    println(shapes)
    println(alpha)
    println(icons)
}
