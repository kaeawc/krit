// Compiler-test source stubs; never packaged in the production artifact.
package android.preference

import android.app.ListActivity
import android.content.Context
import android.content.SharedPreferences

@Deprecated("Deprecated in Java")
abstract class PreferenceActivity : ListActivity() {
    @Deprecated("Deprecated in Java")
    open fun addPreferencesFromResource(preferencesResId: Int) {
        TODO()
    }

    protected open fun isValidFragment(fragmentName: String?): Boolean = TODO()
}

@Deprecated("Deprecated in Java")
object PreferenceManager {
    @Deprecated("Deprecated in Java")
    fun getDefaultSharedPreferences(context: Context?): SharedPreferences = TODO()
}
