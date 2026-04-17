package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun saveTheme(prefs: SharedPreferences, theme: String) {
    prefs.edit().putString("app_theme", theme).apply()
}
