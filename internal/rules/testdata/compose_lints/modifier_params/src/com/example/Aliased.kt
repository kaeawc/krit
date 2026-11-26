package com.example

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.example.callbacks.Listener
import com.example.callbacks.MaybeSelect
import com.example.callbacks.OnSelect

@Composable
fun AliasTrailing(text: String, modifier: Modifier = Modifier, onSelect: OnSelect) {
    Text(text, modifier)
}

@Composable
fun NullableAliasTrailing(text: String, modifier: Modifier = Modifier, onSelect: MaybeSelect) {
    Text(text, modifier)
}

@Composable
fun FunInterfaceLast(text: String, modifier: Modifier = Modifier, listener: Listener) {
    Text(text, modifier)
}

@Composable
fun RunnableLast(text: String, modifier: Modifier = Modifier, action: Runnable) {
    Text(text, modifier)
}
