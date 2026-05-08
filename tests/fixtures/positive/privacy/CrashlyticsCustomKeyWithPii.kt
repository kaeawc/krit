package test

class FirebaseCrashlytics {
    fun setCustomKey(key: String, value: String) {}
    companion object {
        fun getInstance(): FirebaseCrashlytics = FirebaseCrashlytics()
    }
}

fun logCrashInfo(email: String) {
    FirebaseCrashlytics.getInstance().setCustomKey("email", email)
}
