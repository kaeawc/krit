// Smoke: Int/Float/Double .dp and .sp extensions and Dp arithmetic.
package stubs

import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

val Gutter: Dp = 16.dp
val Half: Dp = 0.5.dp
val FloatDp: Dp = 2f.dp
val Sum: Dp = Gutter + 4.dp
val Scaled: Dp = Gutter * 2f
val Doubled: Dp = Gutter * 2
val Divided: Dp = Gutter / 2
val Bigger: Boolean = Gutter > Half
val BodySize: TextUnit = 14.sp
val FloatSp: TextUnit = 1.5f.sp
val Raw: Float = Gutter.value
val Explicit: Dp = Dp(3f)
val Unset: Boolean = Explicit == Dp.Unspecified
