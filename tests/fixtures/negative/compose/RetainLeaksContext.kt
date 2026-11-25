package fixtures.compose

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.retain.retain

class Settings(val name: String)

@Composable
fun Screen(context: Context) {
    val name = context.packageName
    val settings = retain { Settings(name) }
}
