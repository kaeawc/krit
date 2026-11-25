package p

import android.content.Context
import android.content.res.Configuration
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext

@Composable
fun Direct() {
    val config = LocalContext.current.resources.configuration
    val res = LocalContext.current.resources
    val text = LocalContext.current.getString(1)
    val color = LocalContext.current.getColor(1)
    val drawable = LocalContext.current.getDrawable(1)
    val chars = LocalContext.current.getText(1)
    val csl = LocalContext.current.getColorStateList(1)
    val other = LocalContext.current.packageName
    val fqn = androidx.compose.ui.platform.LocalContext.current.resources.configuration
}

@Composable
fun ViaVariable() {
    val context = LocalContext.current
    val config = context.resources.configuration
    val res = context.resources
    val res2 = res.configuration
    val text = context.getString(1)
    val name = context.packageName
}

@Composable
fun ResourcesUse() {
    val res = LocalContext.current.resources
    val s = res.getString(1)
    val d = LocalContext.current.resources.displayMetrics
}

fun NotComposable(context: Context) {
    val config = context.resources.configuration
    val text = context.getString(1)
}

fun nonComposableLocal() {
    val c = LocalContext
}

@Composable
fun Screen() {
    val w = LocalConfiguration.current.screenWidthDp
    val h = LocalConfiguration.current.screenHeightDp
    val config = LocalConfiguration.current
    val w2 = config.screenWidthDp
    val other = LocalConfiguration.current.densityDpi
    val fromContext = LocalContext.current.resources.configuration.screenWidthDp
}

fun nonComposableScreen(config: Configuration): Int = config.screenWidthDp

@Composable
fun composableParam(config: Configuration): Int = config.screenHeightDp

class Holder {
    val lambda = @Composable {
        val w = LocalConfiguration.current.screenWidthDp
    }
}
