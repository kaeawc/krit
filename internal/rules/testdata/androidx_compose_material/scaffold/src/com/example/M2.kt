package com.example

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Scaffold
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun ImplicitIt() {
    Scaffold { Text("x") }
    Scaffold(Modifier, topBar = {}) { Text("x") }
    Scaffold(content = { Text("x") })
}

@Composable
fun Named() {
    Scaffold { padding -> Text("x") }
    Scaffold { _ -> Text("x") }
    Scaffold { `inner` -> Text("x") }
    Scaffold(topBar = { Text("top") }) { innerPadding ->
        Column { Text("x") }
    }
}

@Composable
fun Used() {
    Scaffold { Text("$it") }
    Scaffold { Column(Modifier.padding(it)) {} }
    Scaffold { padding -> Column(Modifier.padding(padding)) {} }
    Scaffold { padding -> Text("${padding}") }
    Scaffold { padding -> Column { Text(padding.toString()) } }
    Scaffold { Column { Text(it.toString()) } }
    Scaffold { -> Text("no params") }
}

@Composable
fun Shadowed(names: List<String>) {
    Scaffold { padding -> names.forEach { padding -> Text(padding) } }
    Scaffold { LazyColumn { items(names) { Text(it) } } }
    Scaffold { names.forEach { Text(it) } }
    Scaffold { padding -> names.forEach { Text(it + padding) } }
    Scaffold { padding -> names.map { (padding) -> padding } }
}

@Composable
fun NonLambda(content: @Composable (PaddingValues) -> Unit) {
    Scaffold(content = content)
    Scaffold(content = fun(p: PaddingValues) { Text("x") })
}
