// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.sensitivekeydivergence

import android.content.Context
import android.content.SharedPreferences

class Holder(private val prefs: SharedPreferences) {
    private val editor: SharedPreferences.Editor = prefs.edit()

    // Go cannot type the receiver of a call chain, so it abstains; the editor is resolved here.
    fun chain(value: String) {
        prefs.edit().putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value).apply()
    }

    // Go cannot type an implicit receiver.
    fun implicitReceiver(value: String) {
        with(prefs.edit()) {
            putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value)
        }
    }

    // A local val is followed to its initializer only when it is encrypted storage.
    fun plainLocal(context: Context, value: String) {
        val local = context.getSharedPreferences("plain", Context.MODE_PRIVATE)
        local.edit().putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value)
    }
}
