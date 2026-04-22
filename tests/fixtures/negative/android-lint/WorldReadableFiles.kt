package com.example

import android.content.Context

class PrefsHelper(private val context: Context) {
    // MODE_WORLD_READABLE is deprecated — do not use this.
    fun getPrefs() = context.getSharedPreferences("data", Context.MODE_PRIVATE)

    fun warning(): String = "Do not use MODE_WORLD_READABLE in your app"
}
