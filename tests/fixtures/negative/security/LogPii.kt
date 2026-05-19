package test

import android.util.Log

class LogPiiSafeFixture {
    fun send(userId: String, password: String) {
        Log.d("Auth", "sending user=$userId")
        Log.d("Auth", "password=<redacted>")
    }
}

// A file-local function that shadows kotlin.io.println. The PII rule must
// not treat the bare call below as a stdout println.
fun println(message: String) {
    // no-op
}

class LogPiiShadowedPrintlnFixture {
    fun emit(password: String) {
        println("password=$password")
    }
}

class LogPiiCustomSink {
    fun println(message: String) {
        // no-op
    }
}

class LogPiiReceiverPrintln {
    fun emit(password: String) {
        val sink = LogPiiCustomSink()
        sink.println("password=$password")
    }
}
