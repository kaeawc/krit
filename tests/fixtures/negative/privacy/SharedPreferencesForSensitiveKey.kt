package test

import android.content.SharedPreferences

fun saveTheme(editor: SharedPreferences.Editor, theme: String) {
    editor.putString("app_theme", theme).apply()
}
