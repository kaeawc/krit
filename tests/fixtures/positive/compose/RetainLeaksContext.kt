package fixtures.compose

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.retain.retain

@Composable
fun Screen(context: Context) {
    val retained = retain { context }
}
