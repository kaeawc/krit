package com.example

import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun M3Screen() {
    Scaffold { Text("x") }
    Scaffold { p -> Text("x") }
    Scaffold { p -> Text("x", Modifier.padding(p)) }
    androidx.compose.material3.Scaffold { Text("x") }
}
