package fixtures.compose

import android.app.Activity
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

@Composable
fun Finish() {
    val activity = LocalContext.current as Activity
    activity.finish()
}
