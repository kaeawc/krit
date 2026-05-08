package com.example

import android.content.Context

class PrefsHelper(private val context: Context) {
    fun getPrefs() = context.getSharedPreferences("data", Context.MODE_WORLD_READABLE)
}
