package test

import android.app.Activity
import android.view.View

class MissingInflatedIdNegative : Activity() {
    fun bind() {
        val v = findViewById<View>(android.R.id.content)
    }
}
