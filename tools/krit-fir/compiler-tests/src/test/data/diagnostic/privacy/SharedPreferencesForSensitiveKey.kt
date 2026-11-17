// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 15, 16, 17, 18, 22, 26, 31, 42, 46
// Positives that match Go: an editor put with a string-literal key that names a secret.
package test.sensitivekey

import android.content.SharedPreferences

fun literals(editor: SharedPreferences.Editor, value: String, count: Int, values: Set<String>) {
    editor.putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value)
    editor.putInt(<!SharedPreferencesForSensitiveKey!>"pin"<!>, count)
    editor.putStringSet(<!SharedPreferencesForSensitiveKey!>"api_keys"<!>, values)
}

fun spellings(editor: SharedPreferences.Editor, value: String) {
    editor.putString(<!SharedPreferencesForSensitiveKey!>"apiKey"<!>, value)
    editor.putString(<!SharedPreferencesForSensitiveKey!>"userPassword"<!>, value)
    editor.putString(<!SharedPreferencesForSensitiveKey!>"""client_secret"""<!>, value)
    editor.putString(<!SharedPreferencesForSensitiveKey!>"OAUTH"<!>, value)
}

fun chained(editor: SharedPreferences.Editor, value: String) {
    editor.putString(<!SharedPreferencesForSensitiveKey!>"refresh_token"<!>, value).apply()
}

fun template(editor: SharedPreferences.Editor, userId: String, value: String) {
    editor.putString(<!SharedPreferencesForSensitiveKey!>"token_$userId"<!>, value)
}

fun multiLine(editor: SharedPreferences.Editor, value: String) {
    editor.putString(
        <!SharedPreferencesForSensitiveKey!>"session_token"<!>,
        value,
    )
}

typealias Alias = SharedPreferences.Editor

class Holder(prefs: SharedPreferences) {
    private val editor: SharedPreferences.Editor = prefs.edit()

    fun fromProperty(value: String) {
        editor.putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value)
    }

    fun aliasedType(alias: Alias, value: String) {
        alias.putString(<!SharedPreferencesForSensitiveKey!>"auth_token"<!>, value)
    }
}
