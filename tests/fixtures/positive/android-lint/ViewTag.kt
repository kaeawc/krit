package test

import android.app.Activity
import android.graphics.drawable.Drawable
import android.view.View

class LeakyTagActivity {
    fun bindActivity(view: View, activity: Activity) {
        view.setTag(activity)
    }

    fun bindDrawable(view: View, drawable: Drawable) {
        view.setTag(
            drawable
        )
    }
}
