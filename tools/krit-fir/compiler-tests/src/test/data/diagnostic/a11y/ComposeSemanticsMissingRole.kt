// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 25, 35, 48, 55, 63, 69, 75
// An interactive modifier on a chain that starts at Modifier gives screen readers no role.
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

@Composable
fun Clickable(onClick: () -> Unit) {
    Row(Modifier.<!ComposeSemanticsMissingRole!>clickable<!> { onClick() }) {}
}

@Composable
fun Toggleable(checked: Boolean, onCheckedChange: (Boolean) -> Unit) {
    Row(
        Modifier.<!ComposeSemanticsMissingRole!>toggleable<!>(
            value = checked,
            onValueChange = onCheckedChange,
        ),
    ) {}
}

@Composable
fun Selectable(selected: Boolean, onClick: () -> Unit) {
    Row(
        Modifier.<!ComposeSemanticsMissingRole!>selectable<!>(
            selected = selected,
            onClick = onClick,
        ),
    ) {}
}

// The finding sits on the line of the modifier name, not on the line the chain starts.
@Composable
fun MultiLineChain(onClick: () -> Unit) {
    Row(
        Modifier
            .padding(8.dp)
            .<!ComposeSemanticsMissingRole!>clickable<!> { onClick() },
    ) {}
}

// A chain handed to then() is read as part of the chain of that call, and semantics without a role does not help.
@Composable
fun ThenChain(onClick: () -> Unit) {
    Row(Modifier.padding(8.dp).then(Modifier.<!ComposeSemanticsMissingRole!>clickable<!> { onClick() })) {}
}

@Composable
fun SemanticsWithoutRole(onClick: () -> Unit) {
    Row(
        Modifier
            .semantics { contentDescription = "Open" }
            .<!ComposeSemanticsMissingRole!>clickable<!> { onClick() },
    ) {}
}

@Composable
fun CompanionRoot(onClick: () -> Unit) {
    Row(Modifier.Companion.<!ComposeSemanticsMissingRole!>clickable<!> { onClick() }) {}
}

@Composable
fun PositionalEnabled(onClick: () -> Unit) {
    // Go reads only a written `enabled = false`, so a positional false is still reported.
    Row(Modifier.<!ComposeSemanticsMissingRole!>clickable<!>(false) { onClick() }) {}
}
