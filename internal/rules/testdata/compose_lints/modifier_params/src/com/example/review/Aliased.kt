package com.example.review

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier as M

// ComposeModifierWithoutDefault: an import alias of Modifier is a Modifier.
@Composable
fun AliasedModifier(text: String, m: M) {
    Text(text, m)
}
