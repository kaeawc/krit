package test

import android.util.Log

class LogPiiSafeFixture {
    fun send(userId: String, password: String) {
        Log.d("Auth", "sending user=$userId")
        Log.d("Auth", "password=<redacted>")
    }
}
