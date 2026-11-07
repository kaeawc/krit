package test

import android.database.Cursor
import android.view.View

class LeakyTagBinder {
    fun bindView(view: View, other: View) {
        view.setTag(1, other)
    }

    fun bindCursor(view: View, cursor: Cursor) {
        view.setTag(
            2,
            cursor
        )
    }
}
