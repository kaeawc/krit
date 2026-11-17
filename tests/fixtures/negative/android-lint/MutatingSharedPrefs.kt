package com.example

import android.content.SharedPreferences

fun addTag(prefs: SharedPreferences) {
    val tags = prefs.getStringSet("tags", null).orEmpty().toMutableSet()
    tags.add("new")
}
