package test;

import android.util.Log;

class LogPiiJavaFixture {
    void send(String authHeader, String userId) {
        Log.d("Auth", "auth=" + authHeader + " user=" + userId);
    }
}
