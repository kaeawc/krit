package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import java.util.Locale

fun language(): String = Locale.getDefault().language

@Composable
fun RememberedLanguage(): String = remember { Locale.getDefault().language }

object Settings {
    fun getDefault(): String = "en"
}

@Composable
fun Setting(): String = Settings.getDefault()
