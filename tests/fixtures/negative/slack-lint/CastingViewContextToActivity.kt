package fixtures.negative.slacklint

import android.app.Activity
import android.content.Context
import android.view.View

class HostView(context: Context) : View(context) {
    // Property access, not a getContext() call, and a cast to Context.
    fun host(): Activity? = context as? Activity
    fun ctx(): Context = getContext() as Context
}
