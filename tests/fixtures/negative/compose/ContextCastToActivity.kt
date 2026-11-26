package fixtures.compose

import android.content.Context
import androidx.activity.compose.LocalActivity
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

class FakeActivity

@Composable
fun Finish(context: Context) {
    val activity = LocalActivity.current
    val notAnActivity = LocalContext.current as Context
    val sameFile = LocalContext.current as FakeActivity
    val other = context as android.app.Activity
}
