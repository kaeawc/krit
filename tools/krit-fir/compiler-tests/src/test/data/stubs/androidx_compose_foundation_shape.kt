// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation.shape

import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

abstract class CornerBasedShape : Shape

class RoundedCornerShape internal constructor() : CornerBasedShape()

// RoundedCornerShape(...) at call sites is a factory function, not a constructor.
fun RoundedCornerShape(size: Dp): RoundedCornerShape = TODO()

fun RoundedCornerShape(percent: Int): RoundedCornerShape = TODO()

fun RoundedCornerShape(
    topStart: Dp = 0.dp,
    topEnd: Dp = 0.dp,
    bottomEnd: Dp = 0.dp,
    bottomStart: Dp = 0.dp,
): RoundedCornerShape = TODO()

val CircleShape: RoundedCornerShape
    get() = TODO()
