package test

object Log {
    fun d(tag: String, msg: String) {}
}

interface SharedPreferences {
    fun getString(key: String, defValue: String?): String?
}

fun debug(prefs: SharedPreferences) {
    Log.d("Auth", prefs.getString("authToken", null) ?: "")
}
