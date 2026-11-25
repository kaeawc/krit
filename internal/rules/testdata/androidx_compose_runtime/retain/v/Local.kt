package v

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.retain.retain
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView

class Holder(val context: Context)

@Composable
fun Locals(holder: Holder) {
    retain { LocalContext.current }
    retain { LocalView.current }
    retain { holder.context }
    val ctx = LocalContext.current
    retain(ctx) { 1 }
}
