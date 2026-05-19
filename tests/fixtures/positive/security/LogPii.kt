package test

import android.util.Log
import timber.log.Timber

class LogPiiFixture {
    fun send(password: String, token: String) {
        Log.d("Auth", "sending password=$password")
        Timber.i("token=$token")
    }
}

class LogPiiStdoutFixture {
    fun emit(password: String) {
        // Bare println in a file that does not shadow kotlin.io.println
        // resolves to stdout and counts as a PII-relevant sink.
        println("password=$password")
        System.out.println("password=$password")
        System.err.println("password=$password")
    }
}
