package com.example

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun MissingRoles(
    checked: Boolean,
    selected: Boolean,
    onClick: () -> Unit,
    onCheckedChange: (Boolean) -> Unit,
) {
    Row(Modifier.clickable { onClick() }) {}

    Row(
        Modifier.toggleable(
            value = checked,
            onValueChange = onCheckedChange,
        ),
    ) {}

    Row(
        Modifier.selectable(
            selected = selected,
            onClick = onClick,
        ),
    ) {}
}
