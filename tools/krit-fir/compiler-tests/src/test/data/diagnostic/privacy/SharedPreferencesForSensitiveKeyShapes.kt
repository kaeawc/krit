// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 21
// Put shapes beyond a plain member call, and encrypted preferences held in a property.
package test.sensitivekeyshapes

import android.content.Context
import android.content.SharedPreferences
import androidx.core.content.edit
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

fun SharedPreferences.Editor.putString(key: String): SharedPreferences.Editor = putString(key, "")

// An extension on the editor writes the preference too, as Go matches the receiver type.
fun extensionPut(editor: SharedPreferences.Editor) {
    editor.putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>)
}

// A `+` chain that starts and ends with a literal is read from its first quote to its last, as Go does.
fun concatenatedKey(editor: SharedPreferences.Editor, id: String, value: String) {
    editor.putString(<!SharedPreferencesForSensitiveKey!>"user_" + id + "_token"<!>, value)
    editor.putString("user_" + id, value)
}

class EncryptedHolder(context: Context) {
    private val prefs: SharedPreferences = EncryptedSharedPreferences.create(
        context,
        "secure",
        MasterKey.Builder(context).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )
    private val lazyPrefs by lazy {
        EncryptedSharedPreferences.create(
            context,
            "secure",
            MasterKey.Builder(context).build(),
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }
    private val editor = prefs.edit()
    private val plain: SharedPreferences = context.getSharedPreferences("plain", Context.MODE_PRIVATE)

    // A property of this file holding encrypted preferences keeps the put encrypted, also inside an editor lambda.
    fun encrypted(value: String) {
        prefs.edit().putString("auth_token", value).apply()
        prefs.edit { putString("auth_token", value) }
        prefs.edit().apply { putString("auth_token", value) }.apply()
        with(prefs.edit()) { putString("auth_token", value) }
        lazyPrefs.edit().putString("auth_token", value).apply()
        editor.putString("auth_token", value)
    }

    // Go cannot type the receiver of a call chain or of an editor lambda, so it abstains.
    fun plainProperty(value: String) {
        plain.edit().putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value).apply()
        plain.edit { putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value) }
    }
}
