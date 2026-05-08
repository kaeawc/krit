package test

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

class UnprotectedDynamicReceiverFixture(
    private val context: Context,
    private val receiver: BroadcastReceiver,
) {
    fun register() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON), null, null)
    }
}
