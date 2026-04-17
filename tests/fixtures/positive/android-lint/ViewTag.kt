package test

import android.view.View

class LeakyTagActivity {
    fun bindView(view: View) {
        view.setTag(activity)
    }
}
