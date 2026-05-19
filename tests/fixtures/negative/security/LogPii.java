package test;

import android.util.Log;

class LogPiiJavaSafeFixture {
    void send(String userId, String token) {
        Log.d("Auth", "user=" + userId);
        Log.d("Auth", "token=<redacted>");
    }
}

class LogPiiJavaCustomSink {
    void println(String message) {
        // no-op
    }

    void emit(String password) {
        LogPiiJavaCustomSink sink = new LogPiiJavaCustomSink();
        sink.println("password=" + password);
    }
}
