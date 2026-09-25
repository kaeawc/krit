// Smoke: Painter is the abstract type painterResource and Coil hand back.
package stubs

import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.res.painterResource

@Composable
fun iconPainter(): Painter = painterResource(android.R.drawable.ic_menu_add)
