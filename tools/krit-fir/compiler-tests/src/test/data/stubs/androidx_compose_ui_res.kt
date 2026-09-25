// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.res

import androidx.annotation.ColorRes
import androidx.annotation.DimenRes
import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.unit.Dp

@Composable
@ReadOnlyComposable
fun stringResource(@StringRes id: Int): String = TODO()

@Composable
@ReadOnlyComposable
fun stringResource(@StringRes id: Int, vararg formatArgs: Any): String = TODO()

@Composable
@ReadOnlyComposable
fun pluralStringResource(id: Int, count: Int, vararg formatArgs: Any): String = TODO()

@Composable
fun painterResource(@DrawableRes id: Int): Painter = TODO()

@Composable
@ReadOnlyComposable
fun colorResource(@ColorRes id: Int): Color = TODO()

@Composable
@ReadOnlyComposable
fun dimensionResource(@DimenRes id: Int): Dp = TODO()
