package com.example

import android.content.SharedPreferences

class Settings(private val prefs: SharedPreferences) {
    fun save(): Boolean {
        prefs.edit().putString("name", "value").apply()
        return prefs.edit().putString("other", "value").commit()
    }
}
