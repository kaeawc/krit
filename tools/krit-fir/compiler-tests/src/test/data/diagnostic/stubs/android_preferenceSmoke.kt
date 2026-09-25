// Smoke: the deprecated framework PreferenceActivity and default shared prefs.
package stubs

import android.app.ListActivity
import android.content.Context
import android.content.SharedPreferences
import android.os.Bundle
import android.preference.PreferenceActivity
import android.preference.PreferenceManager

@Suppress("DEPRECATION")
class SmokeSettingsActivity : PreferenceActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        addPreferencesFromResource(1)
        val list: ListActivity = this
        println(list)
    }

    override fun isValidFragment(fragmentName: String?): Boolean = false
}

@Suppress("DEPRECATION")
fun defaults(context: Context): SharedPreferences = PreferenceManager.getDefaultSharedPreferences(context)
