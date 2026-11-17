package test

import android.content.SharedPreferences
import android.util.Log

fun debug(prefs: SharedPreferences) {
    Log.d("Auth", prefs.getString("authToken", null) ?: "")
}
