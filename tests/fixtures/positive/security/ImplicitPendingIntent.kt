package test

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun insecurePendingIntent(context: Context, intent: Intent) {
    PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
}
