package com.example

import androidx.compose.material.Text
import androidx.compose.material3.Scaffold as M3Scaffold
import androidx.compose.runtime.Composable

@Composable
fun Aliased() {
    M3Scaffold { Text("x") }
    M3Scaffold { padding -> Text("$padding") }
}
