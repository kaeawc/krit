// Smoke: RoundedCornerShape factory overloads and CircleShape.
package stubs

import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.CornerBasedShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.dp

val Rounded: RoundedCornerShape = RoundedCornerShape(8.dp)
val Percent: Shape = RoundedCornerShape(50)
val Corners: CornerBasedShape = RoundedCornerShape(topStart = 4.dp, topEnd = 4.dp)
val Circle: Shape = CircleShape
