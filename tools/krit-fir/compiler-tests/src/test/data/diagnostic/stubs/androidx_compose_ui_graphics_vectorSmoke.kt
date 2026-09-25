// Smoke: ImageVector values from the material icon extensions.
package stubs

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.ui.graphics.vector.ImageVector

val Heart: ImageVector = Icons.Filled.Favorite

fun describe(vector: ImageVector): String = "${vector.name} ${vector.defaultWidth}"
