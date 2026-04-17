package com.example

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun SmallClickableExamples(onTap: () -> Unit) {
    Box(Modifier.size(32.dp).clickable { onTap() })

    Box(
        Modifier
            .height(40.dp)
            .clickable { onTap() },
    )
}
