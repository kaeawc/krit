// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 20, 25, 51, 52, 57
// Read shapes beyond a plain member call, and encrypted preferences held in a property.
package test.logofprefsshapes

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

fun SharedPreferences.getString(key: String): String? = getString(key, null)

fun safeCall(prefs: SharedPreferences?) {
    Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs?.getString("auth_token", null)<!> ?: "")
}

// An extension on SharedPreferences reads the preference too, as Go matches the receiver type.
fun extensionRead(prefs: SharedPreferences) {
    Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("auth_token")<!> ?: "")
}

// A `+` chain that starts and ends with a literal is read from its first quote to its last, as Go does.
fun concatenatedKey(prefs: SharedPreferences, id: String) {
    Log.d("Auth", <!LogOfSharedPreferenceRead!>prefs.getString("user_" + id + "_token", null)<!> ?: "")
    Log.d("Auth", prefs.getString("user_" + id, null) ?: "")
}

class EncryptedHolder(context: Context) {
    private val prefs: SharedPreferences = EncryptedSharedPreferences.create(
        context,
        "secure",
        MasterKey.Builder(context).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )
    private val lazyPrefs: SharedPreferences by lazy {
        EncryptedSharedPreferences.create(
            context,
            "secure",
            MasterKey.Builder(context).build(),
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }
    private val plain: SharedPreferences = context.getSharedPreferences("plain", Context.MODE_PRIVATE)

    // A property of this file holding encrypted preferences keeps the read encrypted. Go reports the first two: it
    // does not follow a property to its initializer or lazy block.
    fun encrypted() {
        Log.d("Auth", prefs.getString("auth_token", null) ?: "")
        Log.d("Auth", lazyPrefs.getString("auth_token", null) ?: "")
        Log.d("Auth", with(prefs) { getString("auth_token", null) } ?: "")
    }

    fun plainProperty() {
        Log.d("Auth", <!LogOfSharedPreferenceRead!>plain.getString("auth_token", null)<!> ?: "")
    }
}
