// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: keys that do not name a secret, constant keys, other owners, and encrypted storage.
package test.sensitivekeynegative

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

const val TOKEN_KEY = "auth_token"

class OtherEditor {
    fun putString(key: String, value: String) {}
}

fun plainKeys(editor: SharedPreferences.Editor, value: String, count: Int) {
    editor.putString("app_theme", value)
    editor.putString("shipping_address", value)
    editor.putInt("spinner_position", count)
    editor.putString("tokenizer_mode", value)
}

// Metadata about a secret names the expiry, count or flag, not the secret.
fun metadata(editor: SharedPreferences.Editor, count: Int) {
    editor.putInt("token_expires_at", count)
    editor.putInt("password_attempts", count)
    editor.putInt("has_pin", count)
}

// Only a string literal key is read; a constant or a computed key is not.
fun notLiteral(editor: SharedPreferences.Editor, key: String, value: String) {
    editor.putString(TOKEN_KEY, value)
    editor.putString(key, value)
}

fun otherOwner(editor: OtherEditor, value: String) {
    editor.putString("auth_token", value)
}

fun otherMethods(editor: SharedPreferences.Editor, value: Boolean) {
    editor.putBoolean("auth_token_seen", value)
    editor.remove("auth_token")
}

fun encrypted(context: Context, value: String) {
    val key = MasterKey.Builder(context).build()
    val prefs = EncryptedSharedPreferences.create(
        context,
        "secure",
        key,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )
    EncryptedSharedPreferences.create(
        context,
        "secure",
        key,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    ).edit().putString("auth_token", value)
}

// A local val holding encrypted preferences keeps the receiver encrypted.
fun encryptedLocal(context: Context, value: String) {
    val key = MasterKey.Builder(context).build()
    val prefs = EncryptedSharedPreferences.create(
        context,
        "secure",
        key,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )
    val editor = prefs.edit()
    editor.putString("auth_token", value)
    prefs.edit().putString("auth_token", value)
}
