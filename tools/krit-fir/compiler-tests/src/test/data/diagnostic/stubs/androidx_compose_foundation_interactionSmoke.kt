// Smoke: an explicit interaction source and a null indication on a clickable modifier.
package stubs

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role

@Composable
fun InteractionSmoke(onDismiss: () -> Unit) {
    val source: InteractionSource = MutableInteractionSource()
    val sink: MutableInteractionSource = MutableInteractionSource()
    Box(Modifier.clickable(interactionSource = sink, indication = null, role = Role.Button) { onDismiss() }) {}
    Box(Modifier.clickable(interactionSource = null, indication = null, enabled = false, onClick = onDismiss)) {}
    <!PrintlnInProduction!>println<!>(source)
}
