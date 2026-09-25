// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation.selection

import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role

fun Modifier.toggleable(
    value: Boolean,
    enabled: Boolean = true,
    role: Role? = null,
    onValueChange: (Boolean) -> Unit,
): Modifier = TODO()

fun Modifier.selectable(
    selected: Boolean,
    enabled: Boolean = true,
    role: Role? = null,
    onClick: () -> Unit,
): Modifier = TODO()

fun Modifier.selectableGroup(): Modifier = TODO()
