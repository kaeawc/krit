// Smoke: the deprecated LocalBroadcastManager singleton.
package stubs

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import androidx.localbroadcastmanager.content.LocalBroadcastManager

@Suppress("DEPRECATION")
fun localBroadcast(context: Context, receiver: BroadcastReceiver) {
    val manager = LocalBroadcastManager.getInstance(context)
    manager.registerReceiver(receiver, IntentFilter("smoke"))
    manager.sendBroadcast(Intent("smoke"))
    manager.unregisterReceiver(receiver)
}
