package com.example

import android.content.SharedPreferences

class Settings(private val prefs: SharedPreferences) {
    fun save() {
        prefs.edit().putString("name", "value").commit()
    }
}
