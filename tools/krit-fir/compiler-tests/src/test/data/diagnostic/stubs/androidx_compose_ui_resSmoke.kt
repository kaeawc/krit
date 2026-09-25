// Smoke: composable resource accessors.
package stubs

import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.res.colorResource
import androidx.compose.ui.res.dimensionResource
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.Dp

@Composable
fun ResourceSmoke(): String {
    val label: String = stringResource(android.R.string.ok)
    val formatted: String = stringResource(android.R.string.ok, "arg", 1)
    val painter: Painter = painterResource(android.R.drawable.ic_menu_add)
    val color: Color = colorResource(android.R.color.black)
    val size: Dp = dimensionResource(1)
    return "$label $formatted $painter $color $size"
}
