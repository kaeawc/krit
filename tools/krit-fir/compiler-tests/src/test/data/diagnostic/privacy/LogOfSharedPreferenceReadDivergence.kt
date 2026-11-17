// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.logofprefsdivergence

import android.content.SharedPreferences
import android.util.Log as AndroidLog
import timber.log.Timber

class Holder(private val prefs: SharedPreferences) {
    // Go needs the receiver to be written as Timber, so it abstains on a Tree reached through a call.
    fun tagged() {
        Timber.tag("Auth").e(<!LogOfSharedPreferenceRead!>prefs.getString("auth_token", null)<!>)
    }

    // Go needs the written receiver Log; an import alias still calls android.util.Log.
    fun aliased() {
        AndroidLog.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("auth_token", null)<!> ?: "")
    }
}
