package test

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter

fun registerUnsafe(context: Context, receiver: BroadcastReceiver) {
    context.registerReceiver(receiver, IntentFilter(Intent.ACTION_SCREEN_ON))
}
