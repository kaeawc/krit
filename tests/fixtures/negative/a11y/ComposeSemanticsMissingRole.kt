package com.example

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics

@Composable
fun ExplicitRoles(
    checked: Boolean,
    selected: Boolean,
    onClick: () -> Unit,
    onCheckedChange: (Boolean) -> Unit,
) {
    Row(Modifier.clickable(role = Role.Button) { onClick() }) {}

    Row(
        Modifier
            .toggleable(
                value = checked,
                onValueChange = onCheckedChange,
            )
            .semantics { role = Role.Switch },
    ) {}

    Row(
        Modifier.selectable(
            selected = selected,
            role = Role.Tab,
            onClick = onClick,
        ),
    ) {}
}

object LocalModifier {
    fun clickable(onClick: () -> Unit): LocalModifier = this
}

fun LocalModifierLookalike(onClick: () -> Unit) {
    LocalModifier.clickable { onClick() }
}

annotation class SignalPreview

@SignalPreview
@Composable
fun RadioRowPreview(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}

@Composable
fun DisabledClickable(onClick: () -> Unit) {
    Row(Modifier.clickable(enabled = false) { onClick() }) {}
}
