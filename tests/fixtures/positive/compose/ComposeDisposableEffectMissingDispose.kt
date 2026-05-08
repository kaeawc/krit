package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect

@Composable
fun Example(source: Source, listener: Listener) {
    DisposableEffect(listener) {
        source.addListener(listener)
    }
}

interface Listener

interface Source {
    fun addListener(listener: Listener)
    fun removeListener(listener: Listener)
}
