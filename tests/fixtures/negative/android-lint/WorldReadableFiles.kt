package com.example

import android.content.Context

class PrefsHelper(private val context: Context) {
    // MODE_WORLD_READABLE is deprecated — do not use this.
    private val warningMsg = "Do not use MODE_WORLD_READABLE in your app"

    fun getPrefs() = context.getSharedPreferences("data", Context.MODE_PRIVATE)
}
