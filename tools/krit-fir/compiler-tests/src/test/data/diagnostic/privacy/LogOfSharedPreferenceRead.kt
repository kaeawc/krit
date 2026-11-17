// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 11, 12, 13, 14, 15, 19, 20, 26, 31x2, 35
// Positives that match Go: a logger call that reads a sensitive-keyed preference in its arguments.
package test.logofprefs

import android.content.SharedPreferences
import android.util.Log

fun levels(prefs: SharedPreferences) {
    Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("authToken", null)<!> ?: "")
    Log.i("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("auth_token", "")<!>.orEmpty())
    Log.w("Auth", <!LogOfSharedPreferenceRead!>prefs.getInt("pin", 0)<!>.toString())
    Log.e("Auth", <!LogOfSharedPreferenceRead!>prefs.getStringSet("api_keys", null)<!>.toString())
    Log.v("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("password", null)<!>.toString())
    Log.wtf("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("client_secret", null)<!>.toString())
}

fun nested(prefs: SharedPreferences) {
    Log.d("Auth", "token=" + <!LogOfSharedPreferenceRead!>prefs.getString("session_token", null)<!>)
    Log.d("Auth", "token=${<!LogOfSharedPreferenceRead!>prefs.getString("session_token", null)<!>}")
}

fun multiLine(prefs: SharedPreferences) {
    Log.d(
        "Auth",
        <!LogOfSharedPreferenceRead!>prefs.getString("refresh_token", null)<!> ?: "",
    )
}

fun twoReads(prefs: SharedPreferences) {
    Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("auth_token", null)<!> + <!LogOfSharedPreferenceRead!>prefs.getString("api_key", null)<!>)
}

fun qualified(prefs: SharedPreferences) {
    android.util.Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("session_token", null)<!> ?: "")
}
