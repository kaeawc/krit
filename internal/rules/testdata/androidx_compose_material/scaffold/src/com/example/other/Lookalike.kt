package com.example.other

import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Scaffold(content: @Composable (Int) -> Unit) = content(1)

@Composable
fun Local() {
    Scaffold { Text("x") }
    Scaffold { p -> Text("x") }
}
