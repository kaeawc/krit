package stubs

import android.view.View
import androidx.compose.runtime.Composable
import androidx.compose.ui.viewinterop.AndroidView

@Composable
fun ViewInteropSmoke() {
    AndroidView(factory = { context -> View(context) })
}
