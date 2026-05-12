package test

object Log {
    fun d(tag: String, msg: String) {}
}

interface SharedPreferences {
    fun getString(key: String, defValue: String?): String?
}

class CustomLogger {
    fun d(tag: String, msg: String) {}
}

// Log.d with no SharedPreferences read: no finding regardless of receiver.
fun debug() {
    Log.d("Auth", "token loaded")
}

// A local val named Log shadows the top-level Log object. In thorough mode
// the resolver sees Log here refers to a CustomLogger value, not the Log
// class, and the rule must suppress the finding even though a sensitive
// getString is passed.
fun shadowed(prefs: SharedPreferences) {
    val Log: CustomLogger = CustomLogger()
    Log.d("Auth", prefs.getString("authToken", null) ?: "")
}
