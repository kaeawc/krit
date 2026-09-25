// Smoke: semantics {} property assignments and Role companion values.
package stubs

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription

@Composable
fun SemanticsSmoke(onOpen: () -> Unit) {
    Box(
        Modifier
            .semantics(mergeDescendants = true) {
                contentDescription = "Card"
                role = Role.Button
                stateDescription = "Selected"
                heading()
                onClick(label = "Open") {
                    onOpen()
                    true
                }
            }
            .clearAndSetSemantics { contentDescription = "Hidden" },
    ) {}
    val roles: List<Role> = listOf(Role.Checkbox, Role.Switch, Role.RadioButton, Role.Tab, Role.Image)
    println(roles)
}
