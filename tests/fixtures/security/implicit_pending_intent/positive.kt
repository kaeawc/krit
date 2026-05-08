package test

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun insecurePendingIntent(context: Context, intent: Intent) {
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
}
