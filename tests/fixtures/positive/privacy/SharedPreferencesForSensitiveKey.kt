package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun saveToken(prefs: SharedPreferences, token: String) {
    prefs.edit().putString("auth_token", token).apply()
}
