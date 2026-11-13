// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Chains that are not reported: an explicit role, a disabled or invisible click target, a chain that sets a role or
// hides itself in semantics, a chain that does not start at Modifier, and preview and fake scaffolding.
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.hideFromAccessibility
import androidx.compose.ui.semantics.invisibleToUser
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.tooling.preview.Preview

@Composable
fun NamedRole(onClick: () -> Unit) {
    Row(Modifier.clickable(role = Role.Button) { onClick() }) {}
}

@Composable
fun PositionalRole(checked: Boolean, onCheckedChange: (Boolean) -> Unit) {
    Row(Modifier.toggleable(checked, true, Role.Switch, onCheckedChange)) {}
}

@Composable
fun NullRole(selected: Boolean, onClick: () -> Unit) {
    Row(Modifier.selectable(selected = selected, role = null, onClick = onClick)) {}
}

@Composable
fun DisabledClickable(onClick: () -> Unit) {
    Row(Modifier.clickable(enabled = false) { onClick() }) {}
}

@Composable
fun InvisibleScrim(onDismiss: () -> Unit) {
    Row(Modifier.clickable(interactionSource = null, indication = null) { onDismiss() }) {}
}

@Composable
fun RoleInSemantics(checked: Boolean, onCheckedChange: (Boolean) -> Unit) {
    Row(
        Modifier
            .toggleable(value = checked, onValueChange = onCheckedChange)
            .semantics { role = Role.Switch },
    ) {}
}

@Composable
fun RoleInSemanticsBeforeClickable(onClick: () -> Unit) {
    Row(Modifier.semantics { role = Role.DropdownList }.clickable { onClick() }) {}
}

@Composable
fun RoleInThenArgument(onClick: () -> Unit) {
    Row(Modifier.then(Modifier.clickable { onClick() }).semantics { role = Role.Tab }) {}
}

@Composable
fun HiddenFromAccessibility(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }.semantics { hideFromAccessibility() }) {}
}

@Composable
fun InvisibleToUser(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }.semantics { invisibleToUser() }) {}
}

@Composable
fun ClearedSemantics(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }.clearAndSetSemantics { contentDescription = "Dismiss" }) {}
}

// A chain that starts at a parameter or a call is not rooted at Modifier.
@Composable
fun ParameterChain(modifier: Modifier, onClick: () -> Unit) {
    Row(modifier.clickable { onClick() }) {}
}

fun customModifier(): Modifier = Modifier

@Composable
fun CallChain(onClick: () -> Unit) {
    Row(customModifier().clickable { onClick() }) {}
}

@Preview
@Composable
fun ClickablePreview(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}

@Composable
fun SampleClickable(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}

@Composable
fun FakeRow(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}

class FakeScreen {
    @Composable
    fun Content(onClick: () -> Unit) {
        Row(Modifier.clickable { onClick() }) {}
    }
}

class ScreenTest {
    @Composable
    fun Content(onClick: () -> Unit) {
        Row(Modifier.clickable { onClick() }) {}
    }
}

// A private helper that only previews call is scaffolding too.
@Composable
private fun PreviewOnlyHelper(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}

@Preview
@Composable
fun HelperPreview() {
    PreviewOnlyHelper {}
}
