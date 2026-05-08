package test

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import androidx.core.content.ContextCompat

class ReceiverSetup(private val context: Context, private val receiver: BroadcastReceiver) {
    fun setup() {
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON), Context.RECEIVER_NOT_EXPORTED)
        context.registerReceiver(receiver, IntentFilter(Intent.ACTION_USER_PRESENT), Context.RECEIVER_EXPORTED or Context.RECEIVER_VISIBLE_TO_INSTANT_APPS)
        ContextCompat.registerReceiver(context, receiver, IntentFilter(Intent.ACTION_SCREEN_ON), ContextCompat.RECEIVER_NOT_EXPORTED)
    }
}

class Registry {
    fun registerReceiver(receiver: Any, filter: Any) {}
}

fun localLookalike(registry: Registry, receiver: Any, filter: Any) {
    registry.registerReceiver(receiver, filter)
}
