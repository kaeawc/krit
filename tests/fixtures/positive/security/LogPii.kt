package test

import android.util.Log
import timber.log.Timber

class LogPiiFixture {
    fun send(password: String, token: String) {
        Log.d("Auth", "sending password=$password")
        Timber.i("token=$token")
    }
}
