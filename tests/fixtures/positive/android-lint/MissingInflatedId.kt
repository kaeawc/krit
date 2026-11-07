package test

import android.app.Activity
import android.view.View

class MissingInflatedIdPositive : Activity() {
    fun bind() {
        setContentView(R.layout.main)
        val v = findViewById<View>(R.id.not_in_layout)
    }
}
