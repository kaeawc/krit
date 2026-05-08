package com.example

import android.content.SharedPreferences

class Prefs(private val prefs: SharedPreferences) {
    fun save(key: String, value: String) {
        val editor = prefs.edit()
        editor.putString(key, value)
        // Missing commit() or apply()
    }
}
