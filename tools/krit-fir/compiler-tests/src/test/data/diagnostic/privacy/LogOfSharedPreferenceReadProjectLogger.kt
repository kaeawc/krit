// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 18, 19
// A project class named Log or Timber is a logger, as Go takes it by its written name.
package test.logofprefsproject

import android.content.SharedPreferences
import timber.log.Timber

object Log {
    fun d(tag: String, msg: String) {}
}

fun projectLogger(prefs: SharedPreferences) {
    Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("authToken", null)<!> ?: "")
}

fun timber(prefs: SharedPreferences) {
    Timber.d(<!LogOfSharedPreferenceRead!>prefs.getString("auth_token", null)<!>)
    Timber.d("token %s", <!LogOfSharedPreferenceRead!>prefs.getString("auth_token", null)<!>)
}
