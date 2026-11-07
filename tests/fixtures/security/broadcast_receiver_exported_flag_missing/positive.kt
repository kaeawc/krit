package test

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter("com.example.SYNC_DONE"))
        context.registerReceiver(receiver, IntentFilter("com.example.REFRESH"), 0)
    }
}
