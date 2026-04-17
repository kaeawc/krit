package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.onDispose

@Composable
fun Example(source: Source, listener: Listener) {
    DisposableEffect(listener) {
        source.addListener(listener)
        onDispose { source.removeListener(listener) }
    }
}

interface Listener

interface Source {
    fun addListener(listener: Listener)
    fun removeListener(listener: Listener)
}
