package test

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON))
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_USER_PRESENT), 0)
    }
}
