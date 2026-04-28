package com.example

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun AccessibleClickableExamples(onTap: () -> Unit) {
    Box(Modifier.size(48.dp).clickable { onTap() })

    Box(Modifier.clickable { onTap() })

    Box(
        Modifier
            .minimumInteractiveComponentSize()
            .clickable { onTap() },
    )
}

object LocalModifier {
    fun size(value: LocalDp): LocalModifier = this
    fun clickable(onClick: () -> Unit): LocalModifier = this
}

class LocalDp
val Int.localDp: LocalDp get() = LocalDp()

fun LocalModifierLookalike(onTap: () -> Unit) {
    LocalModifier.size(24.localDp).clickable { onTap() }
}
