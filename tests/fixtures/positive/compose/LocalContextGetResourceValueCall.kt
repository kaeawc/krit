package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

@Composable
fun Title(): String {
    val context = LocalContext.current
    return context.getString(android.R.string.ok)
}
