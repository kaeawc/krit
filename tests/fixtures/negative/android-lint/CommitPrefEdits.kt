package com.example

import android.content.SharedPreferences

class Prefs(private val prefs: SharedPreferences) {
    fun save(key: String, value: String) {
        prefs.edit().putString(key, value).apply()
    }
}
