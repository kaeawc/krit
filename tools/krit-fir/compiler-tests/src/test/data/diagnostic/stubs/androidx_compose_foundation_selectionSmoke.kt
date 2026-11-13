// Smoke: toggleable/selectable with roles inside a selectable group.
package stubs

import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.selection.toggleable
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role

@Composable
fun SelectionSmoke(checked: Boolean, onChecked: (Boolean) -> Unit, selected: Boolean, onSelect: () -> Unit) {
    Row(Modifier.selectableGroup()) {
        <!ComposeRawTextLiteral!>Text<!>("toggle", Modifier.toggleable(value = checked, role = Role.Switch, onValueChange = onChecked))
        <!ComposeRawTextLiteral!>Text<!>("option", Modifier.selectable(selected = selected, role = Role.RadioButton, onClick = onSelect))
        <!ComposeRawTextLiteral!>Text<!>("plain", Modifier.<!ComposeSemanticsMissingRole!>toggleable<!>(checked) { onChecked(it) })
    }
}
