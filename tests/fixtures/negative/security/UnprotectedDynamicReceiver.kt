package test

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class ProtectedDynamicReceiverFixture(
    private val context: Context,
    private val receiver: BroadcastReceiver,
) {
    fun register() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON), "com.example.PRIVATE", null)
    }
}
