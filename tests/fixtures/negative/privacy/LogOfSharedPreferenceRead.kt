package test

import android.content.SharedPreferences
import android.util.Log

class CustomLogger {
    fun d(tag: String, msg: String) {}
}

// Log.d with no SharedPreferences read: no finding regardless of receiver.
fun debug() {
    Log.d("Auth", "token loaded")
}

// A logger-shaped class that is not Log or Timber is not a logger for this rule.
fun customLogger(prefs: SharedPreferences, logger: CustomLogger) {
    logger.d("Auth", prefs.getString("authToken", null) ?: "")
}

// A non-sensitive key is not reported.
fun theme(prefs: SharedPreferences) {
    Log.d("Theme", prefs.getString("app_theme", null) ?: "")
}
