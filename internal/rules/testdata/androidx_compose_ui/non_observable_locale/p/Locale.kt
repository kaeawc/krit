package p.locale

import android.os.LocaleList
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.core.os.LocaleListCompat
import java.util.Locale

@Composable
fun Formats() {
    val l = Locale.getDefault()
    val list = LocaleList.getAdjustedDefault()
    val compat = LocaleListCompat.getAdjustedDefault()
    val remembered = remember { Locale.getDefault() }
    val fqn = java.util.Locale.getDefault()
    val cat = Locale.getDefault(Locale.Category.FORMAT)
}

fun notComposable() = Locale.getDefault()

@Composable
fun inLambda() {
    val f = { Locale.getDefault() }
    listOf(1).forEach { Locale.getDefault() }
}
