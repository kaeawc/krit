package test

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.PendingIntentCompat

fun securePendingIntent(context: Context, intent: Intent) {
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    PendingIntent.getService(context, 0, intent, flags = PendingIntent.FLAG_MUTABLE)
    PendingIntentCompat.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT, false)
}
