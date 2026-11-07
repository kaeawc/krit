package com.example

import android.app.Activity
import android.view.View

class SettingsActivity : Activity() {
    fun bind() {
        val title = findViewById<View>(R.id.title)
        val subtitle = findViewById<View>(R.id.subtitle)
    }
}
