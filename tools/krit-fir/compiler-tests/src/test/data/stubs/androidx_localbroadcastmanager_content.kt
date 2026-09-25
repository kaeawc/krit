// Compiler-test source stubs; never packaged in the production artifact.
package androidx.localbroadcastmanager.content

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

// Deprecated singleton obtained through the static getInstance(Context).
@Deprecated("Deprecated in Java")
object LocalBroadcastManager {
    fun getInstance(context: Context): LocalBroadcastManager = TODO()

    fun registerReceiver(receiver: BroadcastReceiver, filter: IntentFilter) {
        TODO()
    }

    fun unregisterReceiver(receiver: BroadcastReceiver) {
        TODO()
    }

    fun sendBroadcast(intent: Intent): Boolean = TODO()

    fun sendBroadcastSync(intent: Intent) {
        TODO()
    }
}
