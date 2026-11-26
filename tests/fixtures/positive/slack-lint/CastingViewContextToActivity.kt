package fixtures.positive.slacklint

import android.app.Activity
import android.content.Context
import android.view.View

class HostView(context: Context) : View(context) {
    fun host(): Activity = getContext() as Activity
}
