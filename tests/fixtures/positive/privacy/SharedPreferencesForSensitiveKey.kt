package test

import android.content.SharedPreferences

fun saveToken(editor: SharedPreferences.Editor, token: String) {
    editor.putString("auth_token", token).apply()
}
