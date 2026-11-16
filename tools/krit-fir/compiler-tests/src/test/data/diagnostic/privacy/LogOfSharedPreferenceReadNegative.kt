// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: no preference read, non-secret keys, constant keys, other loggers, and encrypted storage.
package test.logofprefsnegative

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

const val TOKEN_KEY = "auth_token"

class CustomLogger {
    fun d(tag: String, msg: String) {}
}

class Reader {
    fun getString(key: String, defValue: String?): String? = null
}

fun noRead() {
    Log.d("Auth", "token loaded")
}

fun plainKeys(prefs: SharedPreferences) {
    Log.d("Theme", prefs.getString("app_theme", null) ?: "")
    Log.d("Theme", prefs.getString("token_expires_at", null) ?: "")
    Log.d("Theme", prefs.getInt("has_pin", 0).toString())
}

fun notLiteral(prefs: SharedPreferences, key: String) {
    Log.d("Auth", prefs.getString(TOKEN_KEY, null) ?: "")
    Log.d("Auth", prefs.getString(key, null) ?: "")
}

fun otherReader(reader: Reader) {
    Log.d("Auth", reader.getString("auth_token", null) ?: "")
}

fun otherMethods(prefs: SharedPreferences) {
    Log.d("Auth", prefs.getBoolean("auth_token", false).toString())
}

fun shadowedLogger(prefs: SharedPreferences) {
    val Log: CustomLogger = CustomLogger()
    Log.d("Auth", prefs.getString("authToken", null) ?: "")
}

fun readOutsideTheCall(prefs: SharedPreferences) {
    val token = prefs.getString("auth_token", null)
    Log.d("Auth", "loaded")
    println(token)
}

fun encrypted(context: Context) {
    val key = MasterKey.Builder(context).build()
    Log.d(
        "Auth",
        EncryptedSharedPreferences.create(
            context,
            "secure",
            key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        ).getString("auth_token", null) ?: "",
    )
}

fun encryptedLocal(context: Context) {
    val key = MasterKey.Builder(context).build()
    val prefs = EncryptedSharedPreferences.create(
        context,
        "secure",
        key,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )
    Log.d("Auth", prefs.getString("auth_token", null) ?: "")
}
